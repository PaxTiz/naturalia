// texture.glsl samples the painted textures: triplanar projection (no UVs
// needed, works on any shape) and bump mapping from their height channel.
vec3 triWeights(vec3 n) {
    vec3 w = pow(abs(n), vec3(4.0));
    return w / (w.x + w.y + w.z);
}

vec4 tri(sampler2D t, vec3 p, vec3 w, float scale) {
    return texture(t, p.zy * scale) * w.x + texture(t, p.xz * scale) * w.y + texture(t, p.xy * scale) * w.z;
}

// triO samples at p (from origin) plus shift, the tile offset of origin.
vec4 triO(sampler2D t, vec3 p, vec3 shift, vec3 w, float scale) {
    vec3 q = p * scale + shift;
    return texture(t, q.zy) * w.x + texture(t, q.xz) * w.y + texture(t, q.xy) * w.z;
}

// triW samples a world-space texture at the fragment.
vec4 triW(sampler2D t, vec3 w, float scale) { return triO(t, fragRel, fract(origin * scale), w, scale); }

// bump tilts the normal along the slopes of height h (Mikkelsen's
// derivative bump mapping); strength is the relief depth in meters. It
// fades out on faces far from n, like the terrain's skirts seen edge-on,
// where the derivatives give a meaningless tilt.
vec3 bump(vec3 n, vec3 p, float h, float strength) {
    if (strength <= 0.0) return n;
    vec3 dpdx = dFdx(p), dpdy = dFdy(p);
    vec3 r1 = cross(dpdy, n), r2 = cross(n, dpdx);
    float det = dot(dpdx, r1);
    strength *= smoothstep(0.3, 0.6, abs(det) / max(length(cross(dpdx, dpdy)), 1e-12));
    if (strength <= 0.0) return n;
    vec3 grad = sign(det) * (dFdx(h) * r1 + dFdy(h) * r2);
    return normalize(abs(det) * n - strength * grad);
}
