//go:build !js

package gl

import (
	"fmt"
	"strings"
	"unsafe"

	gogl "github.com/go-gl/gl/v3.3-core/gl"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// pipeline holds a compiled and linked GL program with cached
// uniform locations.
type pipeline struct {
	program uint32
	uMVP    int32
	uTM     int32 // texture matrix / params; -1 if unused
	uTex    int32 // sampler uniform; -1 if unused
}

// pipelineSet holds all compiled pipelines.
type pipelineSet struct {
	customCache texcache.Cache[uint64, pipeline]
	solid       pipeline
	shadow      pipeline
	blur        pipeline
	gradient    pipeline
	imageClip   pipeline
	filterBlurH pipeline
	filterBlurV pipeline
	filterColor pipeline
	filterTex   pipeline
	stencil     pipeline
	custom      pipeline // VsCustomGLSL vertex shader, reused per hash
}

func (b *Backend) initPipelines() error {
	type entry struct {
		dst  *pipeline
		vs   string
		fs   string
		name string
	}
	entries := []entry{
		{&b.pipelines.solid, gui.VsGLSL, gui.FsGLSL, "solid"},
		{&b.pipelines.shadow, gui.VsShadowGLSL, gui.FsShadowGLSL, "shadow"},
		{&b.pipelines.blur, gui.VsBlurGLSL, gui.FsBlurGLSL, "blur"},
		{&b.pipelines.gradient, gui.VsGradientGLSL, gui.FsGradientGLSL, "gradient"},
		{&b.pipelines.imageClip, gui.VsGLSL, gui.FsImageClipGLSL, "imageClip"},
		{&b.pipelines.filterBlurH, gui.VsFilterBlurGLSL, gui.FsFilterBlurHGLSL, "filterBlurH"},
		{&b.pipelines.filterBlurV, gui.VsFilterBlurGLSL, gui.FsFilterBlurVGLSL, "filterBlurV"},
		{&b.pipelines.filterColor, gui.VsFilterBlurGLSL, gui.FsFilterColorGLSL, "filterColor"},
		{&b.pipelines.filterTex, gui.VsFilterBlurGLSL, gui.FsFilterTextureGLSL, "filterTex"},
		{&b.pipelines.stencil, gui.VsGLSL, gui.FsStencilGLSL, "stencil"},
		{&b.pipelines.custom, gui.VsCustomGLSL, gui.FsGLSL, "custom"},
	}
	for _, e := range entries {
		p, err := buildPipeline(e.vs, e.fs)
		if err != nil {
			return fmt.Errorf("pipeline %s: %w", e.name, err)
		}
		*e.dst = p
	}
	b.pipelines.customCache = texcache.New[uint64, pipeline](
		maxCustomPipelines, func(p pipeline) {
			if p.program != 0 {
				gogl.DeleteProgram(p.program)
			}
		})
	return nil
}

func (b *Backend) destroyPipelines() {
	destroy := func(p *pipeline) {
		if p.program != 0 {
			gogl.DeleteProgram(p.program)
			p.program = 0
		}
	}
	destroy(&b.pipelines.solid)
	destroy(&b.pipelines.shadow)
	destroy(&b.pipelines.blur)
	destroy(&b.pipelines.gradient)
	destroy(&b.pipelines.imageClip)
	destroy(&b.pipelines.filterBlurH)
	destroy(&b.pipelines.filterBlurV)
	destroy(&b.pipelines.filterColor)
	destroy(&b.pipelines.filterTex)
	destroy(&b.pipelines.stencil)
	destroy(&b.pipelines.custom)
	b.pipelines.customCache.DestroyAll()
}

func buildPipeline(vsSrc, fsSrc string) (pipeline, error) {
	vs, err := compileShader(vsSrc, gogl.VERTEX_SHADER)
	if err != nil {
		return pipeline{}, fmt.Errorf("vertex: %w", err)
	}
	defer gogl.DeleteShader(vs)

	fs, err := compileShader(fsSrc, gogl.FRAGMENT_SHADER)
	if err != nil {
		return pipeline{}, fmt.Errorf("fragment: %w", err)
	}
	defer gogl.DeleteShader(fs)

	prog, err := linkProgram(vs, fs)
	if err != nil {
		return pipeline{}, err
	}

	return pipeline{
		program: prog,
		uMVP:    gogl.GetUniformLocation(prog, glStr("mvp\x00")),
		uTM:     gogl.GetUniformLocation(prog, glStr("tm\x00")),
		uTex:    uniformLoc(prog, "tex\x00", "tex_smp\x00"),
	}, nil
}

func compileShader(src string, shaderType uint32) (uint32, error) {
	shader := gogl.CreateShader(shaderType)
	csrc, free := gogl.Strs(src + "\x00")
	defer free()
	gogl.ShaderSource(shader, 1, csrc, nil)
	gogl.CompileShader(shader)

	var status int32
	gogl.GetShaderiv(shader, gogl.COMPILE_STATUS, &status)
	if status == gogl.FALSE {
		var logLen int32
		gogl.GetShaderiv(shader, gogl.INFO_LOG_LENGTH, &logLen)
		if logLen <= 1 {
			gogl.DeleteShader(shader)
			return 0, fmt.Errorf("compile failed")
		}
		infoLog := make([]byte, logLen)
		gogl.GetShaderInfoLog(shader, logLen, nil, &infoLog[0])
		gogl.DeleteShader(shader)
		return 0, fmt.Errorf("compile: %s", strings.TrimSpace(string(infoLog)))
	}
	return shader, nil
}

func linkProgram(vs, fs uint32) (uint32, error) {
	prog := gogl.CreateProgram()
	gogl.AttachShader(prog, vs)
	gogl.AttachShader(prog, fs)
	gogl.LinkProgram(prog)

	var status int32
	gogl.GetProgramiv(prog, gogl.LINK_STATUS, &status)
	if status == gogl.FALSE {
		var logLen int32
		gogl.GetProgramiv(prog, gogl.INFO_LOG_LENGTH, &logLen)
		if logLen <= 1 {
			gogl.DeleteProgram(prog)
			return 0, fmt.Errorf("link failed")
		}
		infoLog := make([]byte, logLen)
		gogl.GetProgramInfoLog(prog, logLen, nil, &infoLog[0])
		gogl.DeleteProgram(prog)
		return 0, fmt.Errorf("link: %s", strings.TrimSpace(string(infoLog)))
	}
	return prog, nil
}

const maxCustomPipelines = 32

// getOrBuildCustomPipeline returns a cached custom shader pipeline
// or compiles a new one. The cache evicts the LRU entry when full.
func (b *Backend) getOrBuildCustomPipeline(s *gui.Shader) (pipeline, error) {
	h := gui.ShaderHash(s)
	if p, ok := b.pipelines.customCache.Get(h); ok {
		return p, nil
	}
	fsSrc := gui.BuildGLSLFragment(s.GLSL)
	p, err := buildPipeline(gui.VsCustomGLSL, fsSrc)
	if err != nil {
		return pipeline{}, err
	}
	b.pipelines.customCache.Set(h, p)
	return p, nil
}

// usePipeline activates a pipeline and uploads the MVP matrix.
func (b *Backend) usePipeline(p *pipeline) {
	gogl.UseProgram(p.program)
	gogl.UniformMatrix4fv(p.uMVP, 1, false, &b.mvp[0])
}

func glStr(s string) *uint8 {
	return (*uint8)(unsafe.Pointer(unsafe.StringData(s)))
}

func uniformLoc(prog uint32, names ...string) int32 {
	for _, n := range names {
		loc := gogl.GetUniformLocation(prog, glStr(n))
		if loc >= 0 {
			return loc
		}
	}
	return -1
}
