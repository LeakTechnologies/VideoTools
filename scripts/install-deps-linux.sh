#!/bin/bash
# VideoTools Dependency Installer for Linux
# Installs all required build and runtime dependencies

set -e

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Linux Installation"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Detect Linux distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
else
    echo "❌ Cannot detect Linux distribution"
    exit 1
fi

echo "📦 Detected distribution: $DISTRO"
echo ""

# Function to install on Fedora/RHEL/CentOS
install_fedora() {
    echo "Installing dependencies for Fedora/RHEL..."
    sudo dnf install -y \
        gcc \
        pkg-config \
        libX11-devel \
        libXcursor-devel \
        libXrandr-devel \
        libXinerama-devel \
        libXi-devel \
        libXxf86vm-devel \
        mesa-libGL-devel \
        alsa-lib-devel \
        ffmpeg-free \
        golang
    echo "✓ Fedora dependencies installed"
}

# Function to install on Ubuntu/Debian
install_ubuntu() {
    echo "Installing dependencies for Ubuntu/Debian..."
    sudo apt-get update
    sudo apt-get install -y \
        gcc \
        pkg-config \
        libgl1-mesa-dev \
        libx11-dev \
        libxcursor-dev \
        libxrandr-dev \
        libxinerama-dev \
        libxi-dev \
        libxxf86vm-dev \
        libasound2-dev \
        ffmpeg \
        golang-go
    echo "✓ Ubuntu/Debian dependencies installed"
}

# Function to install on Arch Linux
install_arch() {
    echo "Installing dependencies for Arch Linux..."
    sudo pacman -S --needed --noconfirm \
        gcc \
        pkgconf \
        mesa \
        libx11 \
        libxcursor \
        libxrandr \
        libxinerama \
        libxi \
        libxxf86vm \
        alsa-lib \
        ffmpeg \
        go
    echo "✓ Arch Linux dependencies installed"
}

# Function to install on openSUSE
install_opensuse() {
    echo "Installing dependencies for openSUSE..."
    sudo zypper install -y \
        gcc \
        pkg-config \
        Mesa-libGL-devel \
        libX11-devel \
        libXcursor-devel \
        libXrandr-devel \
        libXinerama-devel \
        libXi-devel \
        libXxf86vm-devel \
        alsa-devel \
        ffmpeg \
        go
    echo "✓ openSUSE dependencies installed"
}

# Install based on distribution
case "$DISTRO" in
    fedora|rhel|centos)
        install_fedora
        ;;
    ubuntu|debian|pop|linuxmint)
        install_ubuntu
        ;;
    arch|manjaro|endeavouros)
        install_arch
        ;;
    opensuse*|sles)
        install_opensuse
        ;;
    *)
        echo "❌ Unsupported distribution: $DISTRO"
        echo ""
        echo "Please install these packages manually:"
        echo "  - gcc"
        echo "  - pkg-config"
        echo "  - OpenGL development libraries"
        echo "  - X11 development libraries (libX11, libXcursor, libXrandr, libXinerama, libXi, libXxf86vm)"
        echo "  - ALSA development libraries"
        echo "  - ffmpeg"
        echo "  - Go 1.21 or later"
        exit 1
        ;;
esac

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "✅ DEPENDENCIES INSTALLED"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Verify installations
echo "Verifying installations..."
echo ""

# Check Go
if command -v go &> /dev/null; then
    echo "✓ Go: $(go version)"
else
    echo "⚠️  Go not found in PATH"
fi

# Check GCC
if command -v gcc &> /dev/null; then
    echo "✓ GCC: $(gcc --version | head -1)"
else
    echo "⚠️  GCC not found"
fi

# Check ffmpeg
if command -v ffmpeg &> /dev/null; then
    echo "✓ ffmpeg: $(ffmpeg -version | head -1)"
else
    echo "⚠️  ffmpeg not found in PATH"
fi

# Check pkg-config
if command -v pkg-config &> /dev/null; then
    echo "✓ pkg-config: $(pkg-config --version)"
else
    echo "⚠️  pkg-config not found"
fi

echo ""
echo "Dependencies ready! You can now run:"
echo "  ./scripts/build.sh"
echo ""
