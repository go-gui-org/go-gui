// Concentric rings that drift outward, written as a Shadertoy shader.
// rings_shader.go is generated from this file by cmd/shaderport:
// run `go generate` after editing it.
#define PI 3.14159265

vec3 palette(float t)
{
    return 0.5 + 0.5 * cos(2.0 * PI * (t + vec3(0.0, 0.33, 0.67)));
}

float rings(vec2 p)
{
    float r = length(p);
    return 0.5 + 0.5 * sin(24.0 * r - 3.0 * iTime);
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 p = (2.0 * fragCoord - iResolution.xy) / iResolution.y;
    float a = atan(p.y, p.x) / (2.0 * PI);
    vec3 col = palette(a + 0.1 * iTime) * rings(p);
    fragColor = vec4(col, 1.0);
}
