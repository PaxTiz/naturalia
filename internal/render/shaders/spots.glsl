// The lamps the player built (spots.go): wide cones of light with shadows,
// painted with the same ramp as the sun. Those standing in a building
// light its inside only, the others what is outdoors.
uniform vec4 spotAt[16];  // where, about origin; w how bright
uniform vec4 spotDir[16]; // where to; w the cosine of their edge
uniform vec4 spotCol[16]; // w how far they reach
uniform float spotIn[16]; // 1 in a building
uniform vec4 spotVP[64];  // their cameras about origin, by columns
uniform float spotCount;
uniform sampler2D spotMap; // depth seen from each, in a grid of 4 × 4

vec3 spotLight(vec3 albedo, vec3 n, vec3 face, bool inside) {
    vec3 sum = vec3(0.0);
    for (int i = 0; i < 16; i++) {
        if (float(i) >= spotCount) break;
        if ((spotIn[i] > 0.5) != inside) continue;
        vec3 l = spotAt[i].xyz - fragRel;
        float range = spotCol[i].w, d = length(l);
        if (d > range) continue;
        l /= d;
        float win = clamp(1.0 - pow(d / range, 4.0), 0.0, 1.0);
        float att = spotAt[i].w * win * win * 7.0 / (d * d * 0.5 + 3.0 * max(1.0, spotAt[i].w));
        float edge = smoothstep(spotDir[i].w, mix(spotDir[i].w, 1.0, 0.14), -dot(l, spotDir[i].xyz));
        float lit = edge * lightRamp(dot(n, l));
        if (lit > 0.001) {
            vec4 lp = mat4(spotVP[i * 4], spotVP[i * 4 + 1], spotVP[i * 4 + 2], spotVP[i * 4 + 3]) * vec4(fragRel + face * 0.04, 1.0);
            vec3 sc = lp.xyz / lp.w * 0.5 + 0.5;
            if (lp.w > 0.0 && all(greaterThan(sc.xy, vec2(0.005))) && all(lessThan(sc.xy, vec2(0.995)))) {
                vec2 tc = (vec2(i % 4, i / 4) + sc.xy) * 0.25 * vec2(textureSize(spotMap, 0)) - 0.5;
                vec2 f = fract(tc);
                ivec2 c = ivec2(floor(tc));
                float z = sc.z - 0.00003;
                float s00 = step(z, texelFetch(spotMap, c, 0).r);
                float s10 = step(z, texelFetch(spotMap, c + ivec2(1, 0), 0).r);
                float s01 = step(z, texelFetch(spotMap, c + ivec2(0, 1), 0).r);
                float s11 = step(z, texelFetch(spotMap, c + ivec2(1, 1), 0).r);
                lit *= mix(mix(s00, s10, f.x), mix(s01, s11, f.x), f.y);
            }
        }
        // What the lit floor gives back to the walls and the ceiling around.
        vec3 pool = spotAt[i].xyz + spotDir[i].xyz * 2.2 - fragRel;
        float pd = length(pool);
        float back = 0.28 * max(dot(n, pool / pd), 0.0) * spotAt[i].w * win * 7.0 / (pd * pd + 4.0);
        sum += spotCol[i].rgb * (att * lit + back);
    }
    return albedo * sum;
}
