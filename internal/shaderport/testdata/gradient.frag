// Smallest Shadertoy shape: a time-shifted cosine palette.
void mainImage(out vec4 fragColor, in vec2 fragCoord)
{
    vec2 st = fragCoord / iResolution.xy;
    vec3 col = 0.5 + 0.5 * cos(iTime + st.xyx + vec3(0, 2, 4));
    fragColor = vec4(col, 1.0);
}
