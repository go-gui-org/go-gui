// The frame built-ins, read only from a helper.
#version 300 es
precision highp float;

vec3 flicker()
{
    float on = float(iFrame % 2);
    return vec3(on * clamp(iTimeDelta * 60.0, 0.0, 1.0));
}

void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    fragColor = vec4(flicker() * fragCoord.x / iResolution.x, 1.0);
}
