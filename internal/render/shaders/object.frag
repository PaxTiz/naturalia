#version 330
in vec3 fragPos;
in vec3 fragNormal;
in vec3 localPos;
in vec3 localNormal;
in vec2 fragTexCoord;
uniform vec4 colDiffuse;
uniform vec2 surface;     // x: specular strength, y: emissive (0..1)
uniform float lighting;   // 0 sun, 1 lamps, 2 lamps on back faces, 3 lamps on faces turned toward room
uniform vec3 room;
uniform vec4 lights[24];  // ceiling lamps: xyz position, w intensity
uniform float lightRange[24]; // how far each reaches: its room's walls are in the way
uniform float lightCount;
uniform sampler2D lampShadowMap; // depth seen from the rooms' lamps, a cell of a grid each (shadow.go)
uniform vec4 lampVP[32];         // their cameras about origin, four columns each
uniform float lampCount;         // the first lights, one per room
uniform float lampGrid;          // cells per side of the map
uniform vec2 ground;      // relief shadow and occlusion where the player stands (Terrain.light)
uniform float shadowPass; // 1 while drawing the shadow map: write depth only
uniform vec3 matInfo;     // painted material (mat* constants), texture tiles per meter, 1 if it follows the object
uniform vec2 wear;        // x: dust on the outside of buildings (0..1), y: height of their floor
uniform sampler2D texture0, texture1, texture2, texture3, texture4, texture5, texture6; // texture3: the sun's shadow map
uniform vec4 lightVP[4]; // the sun's shadow camera about origin, by columns
out vec4 finalColor;
#include "common.glsl"
#include "texture.glsl"
// Indoor lighting, the same model as outside: each ceiling lamp lights by
// the inverse square of its distance (softened near it, faded out by its
// range: 10 m in the domes, more in the halls),
// and its light bounces once off the floor and the walls. The floor is
// dark slate and the walls white, so faces turned down get less of it:
// ceilings and undersides sit in a deeper, cooler shade.
const vec3 lampCol = vec3(1.0, 0.9, 0.78);

vec3 indoor(vec3 albedo, vec3 n, vec3 v, float spec, float metal, float ao) {
    float rough = mix(0.85, 0.2, spec);
    vec3 f0 = mix(vec3(0.04), albedo, metal);
    vec3 diff = albedo * (1.0 - metal);
    vec3 col = vec3(0.0);
    float bounce = 0.0;
    for (int i = 0; i < 24; i++) {
        if (float(i) >= lightCount) break;
        vec3 l = (lights[i].xyz - origin) - fragRel;
        float d = length(l);
        // A lamp lights its own room: beyond its range, walls are in the way.
        float range = lightRange[i];
        if (d > range) continue;
        l /= d;
        float win = clamp(1.0 - pow(d / range, 4.0), 0.0, 1.0);
        float att = lights[i].w * win * win / (d * d + 1.2);
        bounce += lights[i].w * win * win / (d * d * 0.3 + 5.0);
        float lit = 1.0;
        if (float(i) < lampCount && att > 0.01) {
            // Pushed a little off the surface, then looked up in the lamp's
            // cell of the map: the four texels around, compared, then
            // blended by where the point falls between them (a soft edge
            // for four reads).
            vec4 lp = mat4(lampVP[i * 4], lampVP[i * 4 + 1], lampVP[i * 4 + 2], lampVP[i * 4 + 3]) * vec4(fragRel + n * 0.04, 1.0);
            vec3 sc = lp.xyz / lp.w * 0.5 + 0.5;
            if (lp.w > 0.0 && all(greaterThan(sc.xy, vec2(0.01))) && all(lessThan(sc.xy, vec2(0.99)))) {
                vec2 size = vec2(textureSize(lampShadowMap, 0));
                int grid = int(lampGrid);
                vec2 tc = (vec2(i % grid, i / grid) + sc.xy) / lampGrid * size - 0.5;
                vec2 f = fract(tc);
                ivec2 c = ivec2(floor(tc));
                float z = sc.z - 0.00002;
                float s00 = step(z, texelFetch(lampShadowMap, c, 0).r);
                float s10 = step(z, texelFetch(lampShadowMap, c + ivec2(1, 0), 0).r);
                float s01 = step(z, texelFetch(lampShadowMap, c + ivec2(0, 1), 0).r);
                float s11 = step(z, texelFetch(lampShadowMap, c + ivec2(1, 1), 0).r);
                lit = mix(mix(s00, s10, f.x), mix(s01, s11, f.x), f.y);
            }
        }
        col += direct(diff, f0, rough, n, v, l, lampCol * att * 11.0 * lit);
    }
    // The bounce: warm off the white walls and the dome above, dim off the
    // slate floor below.
    vec3 above = lampCol * vec3(0.92, 0.9, 0.95) * bounce * 1.5;
    vec3 below = vec3(0.45, 0.43, 0.48) * lampCol * bounce;
    vec3 env = mix(below, above, smoothstep(-0.3, 0.3, reflect(-v, n).y)) * 0.8;
    return col + ambient(diff, f0, rough, n, v, vec3(0, 1, 0), above, below, env, ao);
}

// sunShadow is how much sun reaches the fragment past the base and the
// rocks (the shadow map), softened over 3×3 texels and faded at its edge.
float sunShadow(vec3 n) {
    vec4 lp = mat4(lightVP[0], lightVP[1], lightVP[2], lightVP[3]) * vec4(fragRel + n * 0.16, 1.0);
    vec3 sc = lp.xyz / lp.w * 0.5 + 0.5;
    vec2 border = abs(sc.xy - 0.5) * 2.0;
    float mapped = 1.0 - smoothstep(0.85, 1.0, max(border.x, border.y));
    if (mapped <= 0.0) return 1.0;
    vec2 t = 1.0 / vec2(textureSize(texture3, 0));
    float lit = 0.0;
    for (int y = -1; y <= 1; y++)
        for (int x = -1; x <= 1; x++)
            lit += step(sc.z - 0.0002, texture(texture3, sc.xy + vec2(x, y) * t * 1.3).r);
    return mix(1.0, lit / 9.0, mapped);
}

// paint applies one of the painted materials to the base color; it returns
// the albedo and sets the relief height h (meters) for the bump.
vec3 paint(vec3 base, vec3 n, out float h) {
    h = 0.0;
    if (matInfo.x < 0.5) return base;
    if (matInfo.x > 7.5) return base * texture(texture0, fragTexCoord).rgb; // decal, by UVs
    // World-space textures are laid from origin, shifted back by where
    // origin falls in the tile: the same pattern, without the jitter.
    bool obj = matInfo.z > 0.5;
    vec3 p = obj ? localPos : fragRel;
    vec3 w = triWeights(obj ? normalize(localNormal) : n);
    float s = matInfo.y;
    vec3 shift = obj ? vec3(0.0) : fract(origin * s);
    if (matInfo.x < 1.5) { // factory paint: a smooth, even coat
        vec4 t = triO(texture0, p, shift, w, s);
        return base * (0.95 + 0.06 * t.r);
    }
    if (matInfo.x < 2.5) { // hull plates laid by the mesh's UVs: clean, soft joints
        vec4 t = texture(texture1, fragTexCoord);
        h = t.a * 0.008;
        vec2 cell = floor(fragTexCoord);
        float id = hash13(vec3(cell, floor((fragRel.y + origin.y) * 0.37) + 7.0));
        vec3 c = base * (0.97 + 0.04 * id);
        c = mix(c, c * vec3(0.86, 0.84, 0.82), t.g * (1.0 - abs(n.y)));
        return c * (1.0 - 0.4 * t.b);
    }
    if (matInfo.x < 3.5) { // satin metal: the faintest brushing, no scratches
        vec4 t = triO(texture2, p, shift, w, s);
        return base * (0.9 + 0.14 * t.r);
    }
    if (matInfo.x < 4.5) return base; // the tread plate is gone: its slot holds the sun's shadows
    if (matInfo.x < 5.5) { // fabric
        vec4 t = triO(texture4, p, shift, w, s);
        h = t.a * 0.001;
        return base * (0.85 + 0.2 * t.r);
    }
    if (matInfo.x < 6.5) { // rock: the regolith texture, tinted
        vec4 t = triO(texture5, p, shift, w, s);
        h = t.a * 0.05;
        return base * (0.8 + 0.35 * t.r) * (1.0 - t.b * 0.15);
    }
    // hazard stripes
    vec4 t = triO(texture6, p, shift, w, s);
    vec3 c = mix(vec3(0.08, 0.08, 0.09), vec3(0.95, 0.75, 0.15), t.r);
    return c * (0.75 + 0.35 * t.b);
}

void main() {
    if (shadowPass > 0.5) { finalColor = vec4(gl_FragCoord.z); return; }
    vec3 n = normalize(fragNormal);
    vec3 v = normalize(viewRel - fragRel);
    bool inside = lighting > 0.5 && lighting < 1.5;
    if (lighting > 1.5 && lighting < 2.5 && !gl_FrontFacing) { inside = true; n = -n; }
    if (lighting > 2.5 && dot(n, (room - origin) - fragRel) > 0.0) inside = true;
    float h;
    vec3 albedo = paint(colDiffuse.rgb, n, h);
    if (wear.x > 0.0 && !inside) { // big flat shapes of dust, no grain
        float patch = 0.75 + 0.25 * sin(fragPos.x * 0.7 + 1.3) * sin(fragPos.z * 0.9);
        albedo = mix(albedo, vec3(0.58, 0.31, 0.19), smoothstep(0.55, 0.85, n.y) * 0.18 * patch * wear.x);
        float foot = 1.0 - smoothstep(0.05, 0.55, fragPos.y - wear.y);
        albedo = mix(albedo, albedo * vec3(0.72, 0.52, 0.42), foot * wear.x * 0.4);
    }
    float dist = length(viewRel - fragRel);
    n = bump(n, fragRel, h, 1.0 - smoothstep(10.0, 40.0, dist));
    if (matInfo.x > 5.5 && matInfo.x < 6.5) { // rocks show their facets
        vec3 face = normalize(cross(dFdx(fragRel), dFdy(fragRel)));
        n = face * sign(dot(face, n));
    }
    float metal = matInfo.x > 2.5 && matInfo.x < 3.5 ? 0.55 : 0.0;
    float ao = baseAO(n);
    vec3 lit = inside ? indoor(albedo, n, v, surface.x, metal, ao)
                      : shadeM(albedo, n, v, normalize(fragPos), ground.x * sunShadow(normalize(fragNormal)), mix(0.35, 1.0, ground.y) * ao, surface.x, metal);
    vec3 col = mix(lit, albedo * 2.6, surface.y); // emissive goes past white: it blooms
    finalColor = vec4(finish(col, dist), colDiffuse.a);
}
