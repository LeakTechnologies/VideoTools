#version 330 core

in vec2 TexCoord;
out vec4 FragColor;

uniform sampler2D TextureY;
uniform sampler2D TextureU;
uniform sampler2D TextureV;

uniform mat3 colorMatrix;
uniform float brightness;
uniform float contrast;
uniform float saturation;

vec3 yuv2rgb(vec3 yuv) {
    return colorMatrix * yuv;
}

vec3 adjustBrightness(vec3 color) {
    return color + vec3(brightness);
}

vec3 adjustContrast(vec3 color) {
    return (color - 0.5) * contrast + 0.5;
}

vec3 adjustSaturation(vec3 color, float sat) {
    float gray = dot(color, vec3(0.2126, 0.7152, 0.0722));
    return mix(vec3(gray), color, sat);
}

void main() {
    float y = texture(TextureY, TexCoord).r;
    float u = texture(TextureU, TexCoord).r - 0.5;
    float v = texture(TextureV, TexCoord).r - 0.5;

    vec3 yuv = vec3(y, u, v);
    vec3 rgb = yuv2rgb(yuv);
    
    rgb = adjustBrightness(rgb);
    rgb = adjustContrast(rgb);
    rgb = adjustSaturation(rgb, saturation);
    
    rgb = clamp(rgb, 0.0, 1.0);
    
    FragColor = vec4(rgb, 1.0);
}
