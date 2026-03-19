#version 330 core

in vec2 TexCoord;
out vec4 FragColor;

uniform sampler2D Texture;
uniform float Opacity;

void main() {
    vec4 color = texture(Texture, TexCoord);
    FragColor = vec4(color.rgb, color.a * Opacity);
}
