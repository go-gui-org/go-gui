// Macros: function-like, multi-line, conditional, and macros that
// read built-ins or call a function that does. Names that clash with
// MSL: a user saturate and rotate, and a local named kernel.
#define R iResolution.xy
#define T (iTime * 0.5)
#define ROT(a) mat2(cos(a), -sin(a), \
                    sin(a),  cos(a))
#define WAVE(p) wave(p)

#ifdef GL_ES
#define QUALITY 1
#else
#define QUALITY 2
#endif

float saturate(float x) { return clamp(x, 0.0, 1.0); }

vec2 rotate(vec2 p, float a) { return ROT(a) * p; }

float wave(vec2 p) { return sin(p.x * 10.0 + T) * 0.5 + 0.5; }

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 p = rotate((fragCoord - 0.5 * R) / R.y, T);
    float kernel = saturate(WAVE(p) * float(QUALITY));
    fragColor = vec4(vec3(kernel), 1.0);
}
