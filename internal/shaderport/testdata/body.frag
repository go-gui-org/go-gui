// A go-gui body, not a Shadertoy shader: it declares frag_color and
// reads the wrapper's uv and p0 directly.
vec2 st = uv * 0.5 + 0.5;
float stripes = step(0.5, mod(st.x * 8.0 + iTime, 1.0));
vec4 frag_color = vec4(vec3(stripes) * p0.x, 1.0);
