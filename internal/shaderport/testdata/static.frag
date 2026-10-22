// No time input: the port leaves Animate off.
void mainImage(out vec4 O, vec2 U)
{
    vec2 q = U / iResolution.xy;
    O = vec4(q, 1.0 - q.x, 1.0);
}
