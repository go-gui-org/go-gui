// Value noise and fbm: prototypes, a const matrix, floor/fract/mix and
// a uint hash with bit operations.
float fbm(vec2 p);

const mat2 m2 = mat2(0.8, 0.6, -0.6, 0.8);

uint ihash(uint x)
{
    x ^= x >> 16u;
    x *= 0x7feb352du;
    x ^= x >> 15u;
    return x;
}

float hash(vec2 p)
{
    uvec2 q = uvec2(ivec2(floor(p)) + 1000);
    return float(ihash(q.x + ihash(q.y))) / 4294967295.0;
}

float noise(vec2 p)
{
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash(i), hash(i + vec2(1, 0)), u.x),
               mix(hash(i + vec2(0, 1)), hash(i + vec2(1, 1)), u.x), u.y);
}

float fbm(vec2 p)
{
    float a = 0.5, s = 0.0;
    for (int i = 0; i < 5; i++) {
        s += a * noise(p);
        p = m2 * p * 2.0;
        a *= 0.5;
    }
    return s;
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 p = fragCoord / iResolution.y * 4.0;
    float n = fbm(p + vec2(iTime * 0.2, 0.0));
    fragColor = vec4(vec3(n), 1.0);
}
