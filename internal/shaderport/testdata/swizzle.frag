// Idioms the MSL output must rewrite: stpq swizzles, struct fields
// named like swizzles, inout, component-wise comparisons, two-argument
// atan, mod of a negative value, derivatives, inversesqrt, roundEven,
// and array constructors.
struct Seg { float s; float t; };

const float weights[3] = float[3](0.25, 0.5, 0.25);

void bump(inout vec2 p, float k) { p += vec2(k); }

float blur(float a, float b, float c)
{
    float v[3] = float[](a, b, c);
    float s = 0.0;
    for (int i = 0; i < 3; i++) s += v[i] * weights[i];
    return s;
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 p = fragCoord / iResolution.xy - 0.5;
    bump(p, 0.01);
    Seg seg = Seg(p.s, p.t);
    float ang = atan(p.y, p.x);
    float m = mod(-ang, 1.0);
    bvec2 lt = lessThan(p, vec2(0.0));
    float side = any(not(lt)) ? 1.0 : 0.0;
    float w = fwidth(p.x) + abs(dFdx(p.y)) + abs(dFdy(p.x));
    float r = inversesqrt(dot(p, p) + 1.0) + roundEven(seg.s);
    vec4 c = vec4(m, side, blur(w, r, seg.t), 1.0);
    fragColor = vec4(c.stp, c.q);
}
