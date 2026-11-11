#version 330
in vec3 fragPos;
in vec3 fragNormal;
in float fragSun;
in float fragAO;
in vec3 shadowCoord;
in vec2 fragTexCoord;       // ground kind: x polar ice, y volcanic basalt
uniform sampler2D texture0; // cache of the touched dust tiles
uniform sampler2D texture1; // lookup: where each tile sits in the cache (blue = cached)
uniform float faceCells, tileCells, tilesPerFace, atlasTiles;
uniform float marsRadius;
uniform vec4 brush;         // xyz: aimed point, w: radius (0 hides it)
uniform vec4 casters[16];   // soft shadows of the domes, far away: xyz center, w radius
uniform sampler2D shadowMap; // depth of the objects around the player, seen from the sun
uniform float casterCount;
uniform vec4 steps[48];     // footprints, newest first: xyz position
uniform vec4 stepDirs[48];  // xyz heading, w strength
uniform float stepCount;
uniform sampler2D texture2; // painted regolith: R tone, G pebbles, B hollows, A height
uniform sampler2D texture3; // painted dust: R tone, G clumps, A height
out vec4 finalColor;
#include "common.glsl"
#include "texture.glsl"
const float regolithTile = ${regolithTile};
const float dustTile = ${dustTile};
// Same cube projection as the game code (cubeFaces in mathx.go).
float dustAt(vec3 d) {
    vec3 a = abs(d);
    int f; vec3 N, U, V;
    if (a.x >= a.y && a.x >= a.z) {
        if (d.x > 0.0) { f = 0; N = vec3(1, 0, 0); U = vec3(0, 0, -1); V = vec3(0, 1, 0); }
        else           { f = 1; N = vec3(-1, 0, 0); U = vec3(0, 0, 1); V = vec3(0, 1, 0); }
    } else if (a.y >= a.z) {
        if (d.y > 0.0) { f = 2; N = vec3(0, 1, 0); U = vec3(1, 0, 0); V = vec3(0, 0, -1); }
        else           { f = 3; N = vec3(0, -1, 0); U = vec3(1, 0, 0); V = vec3(0, 0, 1); }
    } else {
        if (d.z > 0.0) { f = 4; N = vec3(0, 0, 1); U = vec3(1, 0, 0); V = vec3(0, 1, 0); }
        else           { f = 5; N = vec3(0, 0, -1); U = vec3(-1, 0, 0); V = vec3(0, 1, 0); }
    }
    float dn = dot(d, N);
    vec2 uv = vec2(atan(dot(d, U) / dn), atan(dot(d, V) / dn)) * (2.0 / 3.14159265) + 0.5;
    // Find the tile, then where the cache keeps it. Untouched tiles are full.
    vec2 c = clamp(uv, 0.0, 1.0) * faceCells;
    vec2 tile = min(floor(c / tileCells), vec2(tilesPerFace - 1.0));
    ivec2 at = ivec2(tile) + ivec2(f - 3 * (f / 3), f / 3) * int(tilesPerFace);
    vec4 entry = texelFetch(texture1, at, 0);
    if (entry.b < 0.5) return 1.0;
    vec2 local = clamp(c - tile * tileCells, 0.5, tileCells - 0.5);
    vec2 slot = floor(entry.rg * 255.0 + 0.5);
    return texture(texture0, (slot * tileCells + local) / (atlasTiles * tileCells)).r;
}

void main() {
    vec3 n = normalize(fragNormal);
    vec3 v = normalize(viewRel - fragRel);
    vec3 up = normalize(fragPos);
    float dist = length(viewRel - fragRel);
    float dust = dustAt(up);

    // Painted textures, projected along the three axes. A second, larger
    // scale fades in with distance to hide the tiling.
    vec3 w = triWeights(n);
    float far = smoothstep(12.0, 60.0, dist) * 0.65;
    vec4 rg = mix(triW(texture2, w, 1.0 / regolithTile), triW(texture2, w, 1.0 / (regolithTile * 7.3)), far);
    vec4 ds = mix(triW(texture3, w, 1.0 / dustTile), triW(texture3, w, 1.0 / (dustTile * 6.1)), far);

    float ice = fragTexCoord.x, basalt = fragTexCoord.y;
    float steep = smoothstep(0.88, 0.7, dot(normalize(fragNormal), up));
    float alt = length(fragPos) - marsRadius;

    // Bare regolith: rusty red, large soft blotches, darker pebbles and
    // hollows. Dark basalt on the volcano; layered rock on steep walls.
    // Big soft-edged patches of flat color, like painted shapes.
    float big = fbm3(fragPos * 0.02);
    big = (floor(big * 4.0) + smoothstep(0.35, 0.65, fract(big * 4.0))) / 4.0;
    vec3 rock = mix(vec3(0.30, 0.13, 0.07), vec3(0.55, 0.27, 0.14), big);
    rock = mix(rock, mix(vec3(0.15, 0.11, 0.10), vec3(0.30, 0.21, 0.17), big), basalt);
    float layer = fract(alt / 9.0 + (fbm3(fragPos * 0.02) - 0.5) * 0.8);
    vec3 strata = mix(vec3(0.62, 0.36, 0.20), vec3(0.36, 0.15, 0.08), smoothstep(0.3, 0.45, layer) * (1.0 - smoothstep(0.75, 0.9, layer)));
    strata = mix(strata, vec3(0.75, 0.55, 0.38), smoothstep(0.92, 0.97, layer) * 0.6);
    rock = mix(rock, mix(strata, strata * vec3(0.55, 0.5, 0.55), basalt), steep);
    rock *= 0.82 + 0.36 * rg.r;
    rock = mix(rock, rock * 0.85, rg.g * 0.5);
    rock *= 1.0 - rg.b * 0.2;
    // Polar ice: bluish white with faint layers.
    vec3 iceCol = mix(vec3(0.60, 0.70, 0.84), vec3(0.84, 0.89, 0.97), smoothstep(0.3, 0.7, fract(alt / 2.5 + big)));
    rock = mix(rock, iceCol * (0.85 + 0.25 * rg.r), ice);

    // Dust settles in the hollows first; pebbles poke through thin layers.
    float edge = (fbm3(fragPos * 1.7) - 0.5) * 0.3;
    float cover = smoothstep(0.08, 0.36, dust + edge + (0.45 - rg.a) * 0.45 * (1.0 - dust));
    vec3 dustCol = mix(vec3(0.70, 0.37, 0.19), vec3(0.84, 0.50, 0.29), smoothstep(0.3, 0.7, fbm3(fragPos * 0.05 + 5.0)));
    dustCol = mix(dustCol, vec3(0.42, 0.27, 0.19), basalt * 0.7);
    float cliff = smoothstep(0.7, 0.5, dot(normalize(fragNormal), up)); // dunes are never this steep
    dustCol = mix(dustCol, strata * 1.15, cliff * 0.45 * (1.0 - basalt));
    // Dust mixed with frost, in big flat patches of bluer, older ice.
    vec3 frost = mix(vec3(0.84, 0.86, 0.93), vec3(0.60, 0.68, 0.84), smoothstep(0.46, 0.52, fbm3(fragPos * 0.045 + 11.0)));
    dustCol = mix(dustCol, frost, ice * 0.95);
    dustCol *= 0.88 + 0.2 * ds.r;
    vec3 albedo = mix(rock, dustCol, cover);
    // A darker rim where dust meets bare ground reads like a thin crust.
    albedo *= 1.0 - 0.22 * cover * (1.0 - cover) * 4.0;

    // Boot prints pressed in the dust: a darker sole with treads, and a
    // lighter rim of pushed-up dust around it.
    if (dist < 40.0 && cover > 0.05) {
        float print = 0.0, rim = 0.0, tread = 0.0;
        for (int i = 0; i < 48; i++) {
            if (float(i) >= stepCount) break;
            vec3 rel = fragRel - (steps[i].xyz - origin);
            if (dot(rel, rel) > 0.09) continue;
            vec3 f = stepDirs[i].xyz;
            vec2 q = vec2(dot(rel, cross(f, up)), dot(rel, f));
            q.x *= 1.0 + 0.35 * smoothstep(0.02, -0.1, q.y); // narrower heel
            float d = length(q / vec2(0.06, 0.15));
            float m = (1.0 - smoothstep(0.8, 1.0, d)) * stepDirs[i].w;
            if (m > print) { print = m; tread = step(0.0, sin(q.y * 120.0)); }
            rim = max(rim, smoothstep(0.85, 1.0, d) * (1.0 - smoothstep(1.0, 1.25, d)) * stepDirs[i].w);
        }
        albedo *= 1.0 - cover * (print * (0.32 + 0.12 * tread) - rim * 0.15);
    }

    // Relief of the textures, fading out with distance.
    float fade = 1.0 - smoothstep(25.0, 70.0, dist);
    float h = mix(rg.a * 0.03, ds.a * 0.012, cover);
    n = bump(n, fragRel, h, fade);
    // Steep relief shows its facets; plains stay smooth.
    vec3 face = normalize(cross(dFdx(fragRel), dFdy(fragRel)));
    face *= sign(dot(face, up));
    n = normalize(mix(n, face, smoothstep(0.25, 0.6, steep) * 0.85));

    // Shadows of the objects around the player, from the shadow map,
    // softened over 3x3 texels and faded out at its edge.
    vec3 sc = shadowCoord;
    vec2 border = abs(sc.xy - 0.5) * 2.0;
    float mapped = 1.0 - smoothstep(0.85, 1.0, max(border.x, border.y));
    float shadow = 1.0;
    if (mapped > 0.0) {
        vec2 t = 1.0 / vec2(textureSize(shadowMap, 0));
        float lit = 0.0;
        for (int y = -1; y <= 1; y++)
            for (int x = -1; x <= 1; x++)
                lit += step(sc.z - 0.0003, texture(shadowMap, sc.xy + vec2(x, y) * t * 1.3).r);
        shadow = mix(1.0, lit / 9.0, mapped);
    }
    // Beyond the map, the domes still cast a soft round shadow.
    for (int i = 0; i < 16; i++) {
        if (float(i) >= casterCount || mapped >= 1.0) break;
        vec3 rel = fragRel - (casters[i].xyz - origin);
        float t = -dot(rel, sunDir);
        if (t <= 0.0) continue;
        float d = length(rel + sunDir * t) / casters[i].w;
        shadow *= mix(1.0, smoothstep(0.3, 1.2, d), 1.0 - mapped);
    }

    // The relief's own shadow and occlusion, baked in the vertices.
    shadow *= fragSun;
    float ao = mix(0.35, 1.0, fragAO) * baseAO(normalize(fragNormal));
    vec3 col = shade(albedo, n, v, up, shadow, ao, mix(0.04 * cover, 0.5, ice));

    if (brush.w > 0.0) {
        float d = length(up - normalize(brush.xyz)) * marsRadius;
        float w = 0.05 + dist * 0.003;
        float ring = 1.0 - smoothstep(0.0, w, abs(d - brush.w));
        float fill = (1.0 - smoothstep(brush.w - 0.6, brush.w, d)) * 0.08;
        col = mix(col, vec3(0.45, 1.0, 0.95), ring * 0.75 + fill);
    }

    finalColor = vec4(finish(col, dist), 1.0);
}
