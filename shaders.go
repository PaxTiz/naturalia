package main

// Every surface is painted by these shaders: no image file is ever loaded.
// World positions are relative to the Moon's center, so normalize(fragPos)
// is the local "up" anywhere on the surface.

// shaderCommon holds the noise, lighting and fog shared by all fragment shaders.
const shaderCommon = `
uniform vec3 viewPos;
uniform vec3 sunDir;
uniform vec3 earthDir;

const vec3 sunCol   = vec3(1.0, 0.94, 0.84);
const vec3 skyFill  = vec3(0.21, 0.23, 0.32);
const vec3 earthCol = vec3(0.28, 0.40, 0.70);
const vec3 rimCol   = vec3(0.45, 0.60, 1.00);
const vec3 horizon  = vec3(0.07, 0.075, 0.15);

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

// Soft stylized lighting: wrapped sun, cool sky fill, earthshine on the
// side facing the Earth, rim light and specular. shadow only dims the sun.
vec3 shade(vec3 albedo, vec3 n, vec3 v, vec3 up, float shadow, float spec) {
    float wrap = clamp((dot(n, sunDir) + 0.3) / 1.3, 0.0, 1.0);
    vec3 col = albedo * (sunCol * wrap * 1.35 * shadow + skyFill * (0.5 + 0.3 * dot(n, up)));
    col += albedo * earthCol * max(dot(n, earthDir), 0.0) * 0.35;
    col += rimCol * pow(1.0 - max(dot(n, v), 0.0), 4.0) * 0.12;
    vec3 h = normalize(sunDir + v);
    col += sunCol * pow(max(dot(n, h), 0.0), 48.0) * spec * shadow;
    return col;
}

vec3 finish(vec3 col, float dist) {
    col = mix(col, horizon, 1.0 - exp(-pow(dist / 1800.0, 2.0)));
    return vec3(1.0) - exp(-col * 1.35); // soft exposure curve, no harsh clipping
}
`

const meshVS = `#version 330
in vec3 vertexPosition;
in vec3 vertexNormal;
uniform mat4 mvp;
uniform mat4 matModel;
uniform mat4 matNormal;
out vec3 fragPos;
out vec3 fragNormal;
void main() {
    fragPos = vec3(matModel * vec4(vertexPosition, 1.0));
    fragNormal = normalize(vec3(matNormal * vec4(vertexNormal, 0.0)));
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

const terrainFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
uniform sampler2D texture0; // cache of the touched dust tiles
uniform sampler2D texture1; // lookup: where each tile sits in the cache (blue = cached)
uniform float faceCells, tileCells, tilesPerFace, atlasTiles;
uniform float moonRadius;
uniform vec4 brush;         // xyz: aimed point, w: radius (0 hides it)
uniform vec4 casters[16];   // soft shadows: xyz center, w radius
uniform float casterCount;
out vec4 finalColor;
` + shaderCommon + `
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
    float dust = dustAt(up);

    // Bare regolith: cool gray with large soft blotches.
    float big = fbm3(fragPos * 0.035);
    vec3 rock = mix(vec3(0.15, 0.155, 0.18), vec3(0.27, 0.27, 0.30), big);
    rock *= 0.9 + 0.1 * vnoise3(fragPos * 1.3);

    // Dust: warm and pale, with organic edges instead of grid cells.
    float edge = (fbm3(fragPos * 2.2) - 0.5) * 0.35;
    float cover = smoothstep(0.10, 0.38, dust + edge);
    vec3 dustCol = mix(vec3(0.72, 0.68, 0.62), vec3(0.86, 0.82, 0.75), fbm3(fragPos * 0.12 + 5.0));
    dustCol *= 0.9 + 0.1 * dust;
    vec3 albedo = mix(rock, dustCol, cover);
    // A darker rim where dust meets bare ground reads like a thin crust.
    albedo *= 1.0 - 0.18 * cover * (1.0 - cover) * 4.0;

    // Soft shadows: is this point behind a caster, as seen from the sun?
    float shadow = 1.0;
    for (int i = 0; i < 16; i++) {
        if (float(i) >= casterCount) break;
        vec3 rel = fragPos - casters[i].xyz;
        float t = -dot(rel, sunDir);
        if (t <= 0.0) continue;
        float d = length(rel + sunDir * t) / casters[i].w;
        shadow *= mix(0.35, 1.0, smoothstep(0.3, 1.2, d));
    }

    vec3 col = shade(albedo, n, v, up, shadow, 0.04 * cover);

    float dist = length(viewPos - fragPos);
    if (brush.w > 0.0) {
        float d = length(up - normalize(brush.xyz)) * moonRadius;
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
uniform vec4 colDiffuse;
uniform vec2 surface;     // x: specular strength, y: emissive (0..1)
uniform float lighting;   // 0 sun, 1 lamps, 2 lamps on back faces, 3 lamps on faces turned toward room
uniform vec3 room;
uniform vec4 lights[12];  // ceiling lamps: xyz position, w intensity
uniform float lightCount;
out vec4 finalColor;
` + shaderCommon + `
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

void main() {
    vec3 n = normalize(fragNormal);
    vec3 v = normalize(viewPos - fragPos);
    bool inside = lighting > 0.5 && lighting < 1.5;
    if (lighting > 1.5 && lighting < 2.5 && !gl_FrontFacing) { inside = true; n = -n; }
    if (lighting > 2.5 && dot(n, room - fragPos) > 0.0) inside = true;
    vec3 lit = inside ? indoor(colDiffuse.rgb, n, v, surface.x) : shade(colDiffuse.rgb, n, v, normalize(fragPos), 1.0, surface.x);
    vec3 col = mix(lit, colDiffuse.rgb * 1.6, surface.y);
    finalColor = vec4(finish(col, length(viewPos - fragPos)), colDiffuse.a);
}`

// holoFS paints the hologram of the Moon above the map table: a glowing
// grid, the cleaned areas, the base and the player. Drawn additively.
const holoFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
uniform sampler2D texture0; // cleaned fraction of each dust tile
uniform vec3 viewPos;
uniform vec3 globeCenter;
uniform mat4 globeRot; // world → Moon frame: turns the player's spot toward the viewer
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
    // Back to the Moon's own frame.
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
uniform vec3 up; // the viewer's local up: the horizon follows them around the Moon
uniform float time;
out vec4 finalColor;
` + shaderCommon + `
vec3 earth(vec3 d, inout float cover) {
    const float dist = 100.0, radius = 7.5;
    vec3 c = earthDir * dist;
    float b = dot(d, c);
    if (b < 0.0) return vec3(0.0);
    float disc = b * b - dot(c, c) + radius * radius;
    // Atmosphere halo just outside the disk.
    float closest = length(d * b - c);
    vec3 halo = vec3(0.35, 0.6, 1.0) * exp(-max(closest - radius, 0.0) * 1.4) * 0.35;
    if (disc < 0.0) return halo;
    vec3 p = d * (b - sqrt(disc));
    vec3 n = normalize(p - c);
    float land = smoothstep(0.52, 0.56, fbm3(n * 2.6 + 3.0));
    vec3 col = mix(vec3(0.06, 0.20, 0.55), vec3(0.20, 0.45, 0.22), land);
    col = mix(col, vec3(0.95, 0.85, 0.65) * 0.8, land * smoothstep(0.62, 0.7, fbm3(n * 5.0)));
    float clouds = smoothstep(0.5, 0.72, fbm3(n * 4.0 + vec3(time * 0.004, 0.0, 0.0)));
    col = mix(col, vec3(0.95), clouds * 0.85);
    // Lit three-quarters toward the viewer, so the Earth reads well in the sky.
    vec3 earthLight = normalize(sunDir - earthDir * 1.2);
    float day = smoothstep(-0.15, 0.35, dot(n, earthLight));
    col *= 0.04 + 1.2 * day;
    col += vec3(0.35, 0.6, 1.0) * pow(1.0 - max(dot(n, -d), 0.0), 2.5) * (0.15 + 0.85 * day);
    cover = smoothstep(0.0, 0.08, disc / radius);
    return mix(halo, col, cover);
}

void main() {
    vec3 d = normalize(fragPos - viewPos);
    float h = dot(d, up);
    vec3 col = mix(horizon, vec3(0.008, 0.009, 0.03), smoothstep(-0.02, 0.5, h));

    // Stars on a fine 3D grid of directions, gently twinkling.
    vec3 p = d * 220.0;
    vec3 cell = floor(p);
    float r = hash13(cell);
    if (r > 0.97) {
        vec3 star = cell + 0.5 + (vec3(hash13(cell + 1.0), hash13(cell + 2.0), hash13(cell + 3.0)) - 0.5) * 0.6;
        float twinkle = 0.7 + 0.3 * sin(time * (1.0 + r * 4.0) + r * 50.0);
        float bright = (r - 0.97) / 0.03;
        col += vec3(0.9, 0.93, 1.0) * smoothstep(0.32, 0.0, length(p - star)) * bright * twinkle * smoothstep(-0.02, 0.12, h);
    }

    float cover = 0.0;
    vec3 e = earth(d, cover);
    col = mix(col, vec3(0.0), cover) + e;

    float s = max(dot(d, sunDir), 0.0);
    col += sunCol * (smoothstep(0.9993, 0.9996, s) * 4.0 + pow(s, 600.0) * 1.5 + pow(s, 14.0) * 0.12);

    finalColor = vec4(vec3(1.0) - exp(-col * 1.35), 1.0);
}`
