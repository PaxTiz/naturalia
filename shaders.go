package main

import "strconv"

// Every surface is painted by these shaders: no image file is ever loaded.
// World positions are relative to Mars' center, so normalize(fragPos)
// is the local "up" anywhere on the surface.

// shaderCommon holds the noise, sky, lighting and haze shared by all
// fragment shaders. The sun is fixed in space: walking around Mars brings
// sunsets and nights, so the sky and the fill light follow the local up.
const shaderCommon = `
uniform vec3 viewPos;
uniform vec3 sunDir;

// Flat, graphic look: a strong warm sun against cool violet shade.
const vec3 sunCol    = vec3(1.05, 0.76, 0.54);
const vec3 skyFill   = vec3(0.20, 0.16, 0.26);
const vec3 nightFill = vec3(0.04, 0.045, 0.09);
const vec3 bounceCol = vec3(0.22, 0.10, 0.05);
const vec3 rimCol    = vec3(1.00, 0.70, 0.50);
const vec3 phobosDir = vec3(0.451, 0.551, -0.702);
const vec3 deimosDir = vec3(-0.204, 0.865, 0.458);

float hash13(vec3 p) {
    p = fract(p * 0.1031);
    p += dot(p, p.zyx + 31.32);
    return fract((p.x + p.y) * p.z);
}

float vnoise3(vec3 p) {
    vec3 i = floor(p), f = fract(p);
    vec3 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(hash13(i), hash13(i + vec3(1, 0, 0)), u.x),
                   mix(hash13(i + vec3(0, 1, 0)), hash13(i + vec3(1, 1, 0)), u.x), u.y),
               mix(mix(hash13(i + vec3(0, 0, 1)), hash13(i + vec3(1, 0, 1)), u.x),
                   mix(hash13(i + vec3(0, 1, 1)), hash13(i + vec3(1, 1, 1)), u.x), u.y), u.z);
}

float fbm3(vec3 p) {
    float s = 0.0, a = 0.5;
    for (int i = 0; i < 5; i++) { s += a * vnoise3(p); p = p * 2.03 + 17.0; a *= 0.5; }
    return s;
}

// daylight: 1 when the sun is well above the local horizon, 0 at night.
float daylight(vec3 up) { return smoothstep(-0.12, 0.18, dot(sunDir, up)); }

// skyColor is the color of the sky seen along d: a clean gradient from a
// peach horizon to a deep dusty rose overhead, bluish around the sun, a
// blue glow at sunset.
vec3 skyColor(vec3 d, vec3 up) {
    float h = dot(d, up);
    float s = max(dot(d, sunDir), 0.0);
    float day = daylight(up);
    vec3 col = mix(vec3(1.00, 0.52, 0.32), vec3(0.36, 0.16, 0.18), smoothstep(-0.02, 0.55, h));
    col = mix(col, vec3(0.89, 1.03, 1.27), pow(s, 16.0) * 0.6);
    col += vec3(1.0, 0.8, 0.6) * pow(s, 4.0) * 0.18;
    vec3 night = mix(vec3(0.05, 0.035, 0.04), vec3(0.008, 0.009, 0.02), smoothstep(-0.05, 0.5, h));
    col = mix(night, col, day);
    float dusk = smoothstep(-0.35, -0.05, dot(sunDir, up)) * (1.0 - day * 0.7);
    col += vec3(0.25, 0.45, 0.95) * pow(s, 6.0) * dusk * 1.4;
    return col;
}

// Graphic stylized lighting: a sun with a crisp but soft terminator, cool
// fill from the sky,
// red light bounced by the ground, rim light and specular. shadow only
// dims the sun, ao (ambient occlusion) only the fill and the bounce.
vec3 shade(vec3 albedo, vec3 n, vec3 v, vec3 up, float shadow, float ao, float spec) {
    float day = daylight(up);
    float wrap = smoothstep(-0.08, 0.3, dot(n, sunDir)) * (0.75 + 0.25 * max(dot(n, sunDir), 0.0));
    vec3 fill = mix(nightFill, skyFill, day);
    // Shade turns a deeper, cooler violet where the sun doesn't reach.
    fill *= mix(vec3(1.0), vec3(0.85, 0.9, 1.3), (1.0 - wrap * shadow) * day);
    vec3 col = albedo * (sunCol * wrap * 1.3 * shadow + fill * (0.55 + 0.3 * dot(n, up)) * ao);
    col += albedo * bounceCol * clamp(0.5 - 0.5 * dot(n, up), 0.0, 1.0) * day * ao;
    col += rimCol * pow(1.0 - max(dot(n, v), 0.0), 4.0) * 0.12 * (0.2 + 0.8 * day);
    vec3 h = normalize(sunDir + v);
    col += sunCol * pow(max(dot(n, h), 0.0), 48.0) * spec * shadow;
    return col;
}

// finish fades distant surfaces into the dusty air: first a violet veil,
// then the sky's own color along the view, so relief stacks up in flat
// layers. The result is HDR: the exposure and the tone curve come later,
// on the whole image (post.go).
vec3 finish(vec3 col, float dist) {
    vec3 d = normalize(fragPos - viewPos);
    vec3 haze = skyColor(d, normalize(viewPos));
    vec3 veil = haze * vec3(0.62, 0.52, 0.72);
    col = mix(col, veil, (1.0 - exp(-dist / 1300.0)) * 0.42);
    return mix(col, haze, 1.0 - exp(-pow(dist / 3300.0, 1.6)));
}
`

const meshVS = `#version 330
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform mat4 mvp;
uniform mat4 matModel;
uniform mat4 matNormal;
out vec3 fragPos;
out vec3 fragNormal;
out vec3 localPos;
out vec3 localNormal;
out vec2 fragTexCoord;
void main() {
    fragTexCoord = vertexTexCoord;
    localPos = vertexPosition;
    localNormal = vertexNormal;
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

// objectVS is meshVS that can also draw instances (DrawMeshInstanced),
// each with its own transform.
const objectVS = `#version 330
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
in mat4 instanceTransform;
uniform mat4 mvp;
uniform mat4 matModel;
uniform mat4 matNormal;
uniform float instanced;
out vec3 fragPos;
out vec3 fragNormal;
out vec3 localPos;
out vec3 localNormal;
out vec2 fragTexCoord;
void main() {
    fragTexCoord = vertexTexCoord;
    localPos = vertexPosition;
    localNormal = vertexNormal;
    if (instanced > 0.5) {
        vec4 p = instanceTransform * vec4(vertexPosition, 1.0);
        fragPos = p.xyz;
        fragNormal = normalize(transpose(inverse(mat3(instanceTransform))) * vertexNormal);
        gl_Position = mvp * p; // mvp is only view and projection here
        return;
    }
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

// terrainVS decodes the light baked in the length of the normals (see
// Terrain.light).
const terrainVS = `#version 330
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
uniform mat4 mvp;
out vec3 fragPos;
out vec3 fragNormal;
out vec2 fragTexCoord;
out float fragSun;
out float fragAO;
void main() {
    float l = length(vertexNormal) - 1.0;
    float occ = floor(l + 0.01);
    fragSun = clamp((l - occ) / 0.9, 0.0, 1.0);
    fragAO = occ / 31.0;
    fragTexCoord = vertexTexCoord;
    fragPos = vertexPosition; // terrain meshes are drawn with no transform
    fragNormal = normalize(vertexNormal);
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

var terrainFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
in float fragSun;
in float fragAO;
in vec2 fragTexCoord;       // ground kind: x polar ice, y volcanic basalt
uniform sampler2D texture0; // cache of the touched dust tiles
uniform sampler2D texture1; // lookup: where each tile sits in the cache (blue = cached)
uniform float faceCells, tileCells, tilesPerFace, atlasTiles;
uniform float marsRadius;
uniform vec4 brush;         // xyz: aimed point, w: radius (0 hides it)
uniform vec4 casters[16];   // soft shadows of the domes, far away: xyz center, w radius
uniform vec4 lightVP[4];    // the shadow map's camera (shadow.go), by columns
uniform sampler2D shadowMap; // depth of the objects around the player, seen from the sun
uniform float casterCount;
uniform vec4 steps[48];     // footprints, newest first: xyz position
uniform vec4 stepDirs[48];  // xyz heading, w strength
uniform float stepCount;
uniform sampler2D texture2; // painted regolith: R tone, G pebbles, B hollows, A height
uniform sampler2D texture3; // painted dust: R tone, G clumps, A height
out vec4 finalColor;
` + shaderCommon + textureCommon + `
const float regolithTile = ` + ftoa(regolithTile) + `;
const float dustTile = ` + ftoa(dustTile) + `;
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
    vec3 v = normalize(viewPos - fragPos);
    vec3 up = normalize(fragPos);
    float dist = length(viewPos - fragPos);
    float dust = dustAt(up);

    // Painted textures, projected along the three axes. A second, larger
    // scale fades in with distance to hide the tiling.
    vec3 w = triWeights(n);
    float far = smoothstep(12.0, 60.0, dist) * 0.65;
    vec4 rg = mix(tri(texture2, fragPos, w, 1.0 / regolithTile), tri(texture2, fragPos, w, 1.0 / (regolithTile * 7.3)), far);
    vec4 ds = mix(tri(texture3, fragPos, w, 1.0 / dustTile), tri(texture3, fragPos, w, 1.0 / (dustTile * 6.1)), far);

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
    vec3 iceCol = mix(vec3(0.70, 0.78, 0.88), vec3(0.92, 0.95, 1.0), smoothstep(0.3, 0.7, fract(alt / 2.5 + big)));
    rock = mix(rock, iceCol * (0.85 + 0.25 * rg.r), ice);

    // Dust settles in the hollows first; pebbles poke through thin layers.
    float edge = (fbm3(fragPos * 1.7) - 0.5) * 0.3;
    float cover = smoothstep(0.08, 0.36, dust + edge + (0.45 - rg.a) * 0.45 * (1.0 - dust));
    vec3 dustCol = mix(vec3(0.74, 0.43, 0.24), vec3(0.88, 0.58, 0.36), fbm3(fragPos * 0.12 + 5.0));
    dustCol = mix(dustCol, vec3(0.42, 0.27, 0.19), basalt * 0.7);
    dustCol = mix(dustCol, strata * 1.15, steep * 0.45 * (1.0 - basalt));
    dustCol = mix(dustCol, vec3(0.97, 0.93, 0.92), ice * 0.95); // dust mixed with frost
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
            vec3 rel = fragPos - steps[i].xyz;
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
    n = bump(n, fragPos, h, fade);
    // Steep relief shows its facets; plains stay smooth.
    vec3 face = normalize(cross(dFdx(fragPos), dFdy(fragPos)));
    face *= sign(dot(face, up));
    n = normalize(mix(n, face, smoothstep(0.25, 0.6, steep) * 0.85));

    // Shadows of the objects around the player, from the shadow map,
    // softened over 3x3 texels and faded out at its edge.
    vec4 lp = mat4(lightVP[0], lightVP[1], lightVP[2], lightVP[3]) * vec4(fragPos, 1.0);
    vec3 sc = lp.xyz / lp.w * 0.5 + 0.5;
    vec2 border = abs(sc.xy - 0.5) * 2.0;
    float mapped = 1.0 - smoothstep(0.85, 1.0, max(border.x, border.y));
    float shadow = 1.0;
    if (mapped > 0.0) {
        vec2 t = 1.0 / vec2(textureSize(shadowMap, 0));
        float lit = 0.0;
        for (int y = -1; y <= 1; y++)
            for (int x = -1; x <= 1; x++)
                lit += step(sc.z - 0.0003, texture(shadowMap, sc.xy + vec2(x, y) * t * 1.3).r);
        shadow = mix(1.0, mix(0.35, 1.0, lit / 9.0), mapped);
    }
    // Beyond the map, the domes still cast a soft round shadow.
    for (int i = 0; i < 16; i++) {
        if (float(i) >= casterCount || mapped >= 1.0) break;
        vec3 rel = fragPos - casters[i].xyz;
        float t = -dot(rel, sunDir);
        if (t <= 0.0) continue;
        float d = length(rel + sunDir * t) / casters[i].w;
        shadow *= mix(1.0, mix(0.35, 1.0, smoothstep(0.3, 1.2, d)), 1.0 - mapped);
    }

    // The relief's own shadow and occlusion, baked in the vertices.
    shadow *= fragSun;
    float ao = mix(0.35, 1.0, fragAO);
    vec3 col = shade(albedo, n, v, up, shadow, ao, mix(0.04 * cover, 0.6, ice * (1.0 - cover)));

    if (brush.w > 0.0) {
        float d = length(up - normalize(brush.xyz)) * marsRadius;
        float w = 0.05 + dist * 0.003;
        float ring = 1.0 - smoothstep(0.0, w, abs(d - brush.w));
        float fill = (1.0 - smoothstep(brush.w - 0.6, brush.w, d)) * 0.08;
        col = mix(col, vec3(0.45, 1.0, 0.95), ring * 0.75 + fill);
    }

    finalColor = vec4(finish(col, dist), 1.0);
}`

const objectFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
in vec3 localPos;
in vec3 localNormal;
in vec2 fragTexCoord;
uniform vec4 colDiffuse;
uniform vec2 surface;     // x: specular strength, y: emissive (0..1)
uniform float lighting;   // 0 sun, 1 lamps, 2 lamps on back faces, 3 lamps on faces turned toward room
uniform vec3 room;
uniform vec4 lights[12];  // ceiling lamps: xyz position, w intensity
uniform float lightCount;
uniform vec2 ground;      // relief shadow and occlusion where the player stands (Terrain.light)
uniform float shadowPass; // 1 while drawing the shadow map: write depth only
uniform vec3 matInfo;     // painted material (mat* constants), texture tiles per meter, 1 if it follows the object
uniform sampler2D texture0, texture1, texture2, texture3, texture4, texture5, texture6;
out vec4 finalColor;
` + shaderCommon + textureCommon + `
// Warm, soft indoor lighting from the ceiling lamps.
vec3 indoor(vec3 albedo, vec3 n, vec3 v, float spec) {
    const vec3 lampCol = vec3(1.0, 0.9, 0.76);
    vec3 col = albedo * vec3(0.30, 0.30, 0.34);
    for (int i = 0; i < 12; i++) {
        if (float(i) >= lightCount) break;
        vec3 l = lights[i].xyz - fragPos;
        float d = length(l);
        l /= d;
        float att = lights[i].w / (1.0 + 0.12 * d * d);
        float wrap = clamp((dot(n, l) + 0.4) / 1.4, 0.0, 1.0);
        col += albedo * lampCol * wrap * att * 1.4;
        col += lampCol * pow(max(dot(n, normalize(l + v)), 0.0), 40.0) * spec * att * 0.6;
    }
    return col;
}

// paint applies one of the painted materials to the base color; it returns
// the albedo and sets the relief height h (meters) for the bump.
vec3 paint(vec3 base, vec3 n, out float h) {
    h = 0.0;
    if (matInfo.x < 0.5) return base;
    if (matInfo.x > 7.5) return base * texture(texture0, fragTexCoord).rgb; // decal, by UVs
    vec3 p = matInfo.z > 0.5 ? localPos : fragPos;
    vec3 w = triWeights(matInfo.z > 0.5 ? normalize(localNormal) : n);
    float s = matInfo.y;
    if (matInfo.x < 1.5) { // painted object: brush strokes, chipped edges showing metal
        vec4 t = tri(texture0, p, w, s);
        h = t.a * 0.004;
        return mix(base * t.r, vec3(0.42, 0.42, 0.46) * t.r, t.b * 0.8);
    }
    if (matInfo.x < 2.5) { // hull panels: seams, rivets, grime streaks, worn edges
        vec4 t = tri(texture1, p, w, s);
        h = t.a * 0.02;
        vec3 c = base * t.r;
        c = mix(c, c * vec3(0.55, 0.5, 0.45), t.g * 0.75);
        return mix(c, vec3(0.62, 0.64, 0.68), t.b * 0.6);
    }
    if (matInfo.x < 3.5) { // brushed metal with scratches
        vec4 t = tri(texture2, p, w, s);
        h = t.a * 0.003;
        return base * (0.55 + 0.7 * t.r) + t.g * 0.15;
    }
    if (matInfo.x < 4.5) { // tread plate floor
        vec4 t = tri(texture3, p, w, s);
        h = t.a * 0.008;
        return base * (0.45 + 0.9 * t.r) * (1.0 - 0.4 * t.g);
    }
    if (matInfo.x < 5.5) { // fabric
        vec4 t = tri(texture4, p, w, s);
        h = t.a * 0.003;
        return base * (0.55 + 0.55 * t.r);
    }
    if (matInfo.x < 6.5) { // rock: the regolith texture, tinted
        vec4 t = tri(texture5, p, w, s);
        h = t.a * 0.05;
        return base * (0.8 + 0.35 * t.r) * (1.0 - t.b * 0.15);
    }
    // hazard stripes
    vec4 t = tri(texture6, p, w, s);
    vec3 c = mix(vec3(0.08, 0.08, 0.09), vec3(0.95, 0.75, 0.15), t.r);
    return c * (0.75 + 0.35 * t.b);
}

void main() {
    if (shadowPass > 0.5) { finalColor = vec4(gl_FragCoord.z); return; }
    vec3 n = normalize(fragNormal);
    vec3 v = normalize(viewPos - fragPos);
    bool inside = lighting > 0.5 && lighting < 1.5;
    if (lighting > 1.5 && lighting < 2.5 && !gl_FrontFacing) { inside = true; n = -n; }
    if (lighting > 2.5 && dot(n, room - fragPos) > 0.0) inside = true;
    float h;
    vec3 albedo = paint(colDiffuse.rgb, n, h);
    float dist = length(viewPos - fragPos);
    n = bump(n, fragPos, h, 1.0 - smoothstep(10.0, 40.0, dist));
    if (matInfo.x > 5.5 && matInfo.x < 6.5) { // rocks show their facets
        vec3 face = normalize(cross(dFdx(fragPos), dFdy(fragPos)));
        n = face * sign(dot(face, n));
    }
    vec3 lit = inside ? indoor(albedo, n, v, surface.x) : shade(albedo, n, v, normalize(fragPos), ground.x, mix(0.35, 1.0, ground.y), surface.x);
    vec3 col = mix(lit, albedo * 2.6, surface.y); // emissive goes past white: it blooms
    finalColor = vec4(finish(col, dist), colDiffuse.a);
}`

// holoFS paints the hologram of Mars above the map table: a glowing
// grid, the cleaned areas, the base and the player. Drawn additively.
const holoFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
uniform sampler2D texture0; // cleaned fraction of each dust tile
uniform vec3 viewPos;
uniform vec3 globeCenter;
uniform mat4 globeRot; // world → Mars frame: turns the player's spot toward the viewer
uniform vec3 playerDir;
uniform float tilesPerFace;
uniform float time;
out vec4 finalColor;

float coverage(vec3 d) {
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
    vec2 tile = min(floor(clamp(uv, 0.0, 1.0) * tilesPerFace), vec2(tilesPerFace - 1.0));
    return texelFetch(texture0, ivec2(tile) + ivec2(f - 3 * (f / 3), f / 3) * int(tilesPerFace), 0).r;
}

void main() {
    // Back to Mars' own frame.
    vec3 w = normalize(fragPos - globeCenter);
    vec3 d = normalize((globeRot * vec4(w, 0.0)).xyz);

    vec3 v = normalize(viewPos - fragPos);
    float rim = pow(1.0 - abs(dot(normalize(fragNormal), v)), 2.0);
    vec3 col = vec3(0.10, 0.45, 0.65) * (0.22 + rim * 0.9);

    float lat = asin(clamp(d.y, -1.0, 1.0)) / 0.2618;
    float lon = atan(d.z, d.x) / 0.2618;
    float grid = max(1.0 - smoothstep(0.0, 0.06, abs(fract(lat + 0.5) - 0.5)),
                     (1.0 - smoothstep(0.0, 0.06, abs(fract(lon + 0.5) - 0.5))) * sqrt(max(0.0, 1.0 - d.y * d.y)));
    col += vec3(0.20, 0.65, 0.90) * grid * 0.3;

    col = mix(col, vec3(0.75, 1.0, 1.0) * 1.3, clamp(coverage(d) * 3.0, 0.0, 1.0));
    col += vec3(1.0, 0.8, 0.3) * smoothstep(0.996, 0.999, d.y) * 2.0;
    float pulse = 0.6 + 0.4 * sin(time * 6.0);
    col += vec3(1.0, 0.35, 0.25) * smoothstep(0.997, 0.9993, dot(d, playerDir)) * 2.0 * pulse;
    col *= 0.85 + 0.15 * sin(fragPos.y * 90.0 + time * 3.0);
    finalColor = vec4(col, 1.0);
}`

const skyFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
uniform vec3 up; // the viewer's local up: the horizon follows them around Mars
uniform float time;
out vec4 finalColor;
` + shaderCommon + `
// moon draws one of the two small moons: a lumpy gray rock, lit by the
// sun. It returns its color and sets cover where it hides the sky.
vec3 moon(vec3 d, vec3 dir, float radius, float seed, inout float cover) {
    float c = dot(d, dir);
    if (c < 0.995) return vec3(0.0);
    // Position on the disk, in units of its radius.
    vec3 t1 = normalize(cross(dir, vec3(0.0, 1.0, 0.0)));
    vec3 t2 = cross(t1, dir);
    vec2 q = vec2(dot(d, t1), dot(d, t2)) / radius;
    float a = atan(q.y, q.x);
    float edge = 1.0 + 0.16 * (vnoise3(vec3(cos(a), sin(a), seed) * 1.6) - 0.5) * 2.0;
    float r = length(q) / edge;
    float k = 1.0 - smoothstep(0.92, 1.0, r);
    if (k <= 0.0) return vec3(0.0);
    vec3 n = normalize(t1 * q.x + t2 * q.y - dir * sqrt(max(0.0, 1.0 - r * r)));
    // Lit three-quarters toward the viewer, so the moons read well in the sky.
    float lit = clamp((dot(n, normalize(sunDir - dir * 1.2)) + 0.15) / 1.15, 0.0, 1.0);
    float craters = vnoise3(vec3(q * 3.0, seed)) * 0.5 + vnoise3(vec3(q * 7.0, seed + 3.0)) * 0.25;
    vec3 col = vec3(0.55, 0.50, 0.46) * (0.6 + 0.6 * craters) * (0.08 + 1.1 * lit);
    cover = k;
    return col * k;
}

void main() {
    vec3 d = normalize(fragPos - viewPos);
    float h = dot(d, up);
    vec3 col = skyColor(d, up);
    float day = daylight(up);

    // Stars come out at night only, on a fine 3D grid of directions.
    vec3 p = d * 220.0;
    vec3 cell = floor(p);
    float r = hash13(cell);
    if (r > 0.97) {
        vec3 star = cell + 0.5 + (vec3(hash13(cell + 1.0), hash13(cell + 2.0), hash13(cell + 3.0)) - 0.5) * 0.6;
        float twinkle = 0.7 + 0.3 * sin(time * (1.0 + r * 4.0) + r * 50.0);
        float bright = (r - 0.97) / 0.03;
        col += vec3(0.9, 0.93, 1.0) * smoothstep(0.32, 0.0, length(p - star)) * bright * twinkle * smoothstep(-0.02, 0.12, h) * (1.0 - day);
    }

    // Phobos, then tiny Deimos: pale in the day sky, bright at night.
    float cover = 0.0;
    vec3 m = moon(d, phobosDir, 0.04, 1.0, cover);
    col = mix(col, m + col * (0.55 * day), cover);
    cover = 0.0;
    m = moon(d, deimosDir, 0.015, 7.0, cover);
    col = mix(col, m + col * (0.55 * day), cover);

    // The sun looks smaller from Mars, with a soft glow in the dusty air.
    float s = max(dot(d, sunDir), 0.0);
    col += sunCol * (smoothstep(0.99975, 0.99988, s) * 5.0 + pow(s, 900.0) * 1.2 + pow(s, 40.0) * 0.15);

    finalColor = vec4(col, 1.0);
}`

// textureCommon samples the painted textures: triplanar projection (no UVs
// needed, works on any shape) and bump mapping from their height channel.
const textureCommon = `
vec3 triWeights(vec3 n) {
    vec3 w = pow(abs(n), vec3(4.0));
    return w / (w.x + w.y + w.z);
}

vec4 tri(sampler2D t, vec3 p, vec3 w, float scale) {
    return texture(t, p.zy * scale) * w.x + texture(t, p.xz * scale) * w.y + texture(t, p.xy * scale) * w.z;
}

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
`

// ftoa writes a Go constant as a GLSL float literal.
func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }
