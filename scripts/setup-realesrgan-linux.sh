#!/bin/bash
# Quick Real-ESRGAN setup for Linux
set -e

INSTALL_DIR="$HOME/.local/bin"
MODELS_DIR="$HOME/.local/share/realesrgan/models"

mkdir -p "$INSTALL_DIR"
mkdir -p "$MODELS_DIR"

echo "════════════════════════════════════════════════════════════════"
echo "  Real-ESRGAN ncnn Setup for Linux"
echo "════════════════════════════════════════════════════════════════"
echo ""

cd /tmp

echo "📥 Downloading Real-ESRGAN ncnn Vulkan..."
if ! wget -q --show-progress -O realesrgan-ncnn-vulkan.zip \
  https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-ubuntu.zip; then
    echo "❌ Download failed. Please check your internet connection."
    exit 1
fi

echo ""
echo "📦 Extracting..."
unzip -o realesrgan-ncnn-vulkan.zip > /dev/null

echo "📂 Installing to $INSTALL_DIR..."
cp realesrgan-ncnn-vulkan "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/realesrgan-ncnn-vulkan"

echo "📂 Installing models to $MODELS_DIR..."
cp -r models/* "$MODELS_DIR/"

echo "🧹 Cleaning up..."
rm -rf realesrgan-ncnn-vulkan.zip realesrgan-ncnn-vulkan models/ input.jpg input2.jpg onepiece_demo.mp4 README_ubuntu.md

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "✅ Real-ESRGAN Successfully Installed!"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Binary: $INSTALL_DIR/realesrgan-ncnn-vulkan"
echo "Models: $MODELS_DIR"
echo ""
echo "Test it:"
echo "  realesrgan-ncnn-vulkan -v"
echo ""
echo "Note: Make sure $INSTALL_DIR is in your PATH"
echo "Add this to your ~/.bashrc if needed:"
echo '  export PATH="$HOME/.local/bin:$PATH"'
echo ""
