// A sphere-and-floor raymarcher: structs, out parameters, loops, an
// early return, a look-at matrix and helpers that read iTime.
#define MAX_STEPS 64
#define EPS 0.001

const float FAR = 20.0;

struct Ray { vec3 ro; vec3 rd; };

float sdSphere(vec3 p, float r) { return length(p) - r; }

float map(vec3 p)
{
    vec3 c = vec3(0.0, 1.0 + 0.25 * sin(iTime), 0.0);
    return min(sdSphere(p - c, 1.0), p.y);
}

void calcNormal(in vec3 p, out vec3 n)
{
    const vec2 e = vec2(EPS, 0.0);
    n = normalize(vec3(map(p + e.xyy) - map(p - e.xyy),
                       map(p + e.yxy) - map(p - e.yxy),
                       map(p + e.yyx) - map(p - e.yyx)));
}

mat3 lookAt(vec3 eye, vec3 target)
{
    vec3 f = normalize(target - eye);
    vec3 r = normalize(cross(vec3(0, 1, 0), f));
    return mat3(r, cross(f, r), f);
}

bool march(Ray ray, out float t)
{
    t = 0.0;
    for (int i = 0; i < MAX_STEPS; i++) {
        float d = map(ray.ro + ray.rd * t);
        if (d < EPS) return true;
        t += d;
        if (t > FAR) break;
    }
    return false;
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 p = (2.0 * fragCoord - iResolution.xy) / iResolution.y;
    vec3 eye = vec3(4.0 * cos(0.2 * iTime), 2.0, 4.0 * sin(0.2 * iTime));
    Ray ray = Ray(eye, lookAt(eye, vec3(0, 1, 0)) * normalize(vec3(p, 1.5)));
    float t;
    if (!march(ray, t)) {
        fragColor = vec4(0.1, 0.1, 0.15, 1.0);
        return;
    }
    vec3 n;
    calcNormal(ray.ro + ray.rd * t, n);
    float diff = max(dot(n, normalize(vec3(1, 2, -1))), 0.0);
    fragColor = vec4(vec3(diff), 1.0);
}
