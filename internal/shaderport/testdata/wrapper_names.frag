// Program-scope names the go-gui wrapper also declares: a palette
// function named color and a constant named tex. Both get renamed.
const float tex = 0.75;

vec3 color(float t)
{
    return 0.5 + 0.5 * cos(6.28318 * (t + vec3(0.0, 0.33, 0.67)));
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 uv = fragCoord / iResolution.xy;
    fragColor = vec4(color(uv.x) * tex, 1.0);
}
