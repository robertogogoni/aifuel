# Maintainer: Roberto Gogoni <robertogogoni@outlook.com>
pkgname=aifuel-bin
pkgver=1.5.0
pkgrel=1
pkgdesc="Real-time AI provider usage monitor for waybar (Claude, Codex, Gemini, Copilot)"
arch=('x86_64' 'aarch64')
url="https://github.com/robertogogoni/aifuel"
license=('MIT')
depends=('jq' 'curl' 'waybar')
optdepends=(
    'libnotify: desktop notifications'
    'google-chrome: Chrome extension live feed'
    'chromium: Chrome extension live feed'
    'gum: enhanced terminal prompts'
)
provides=('aifuel')
conflicts=('aifuel')

source_x86_64=("${url}/releases/download/v${pkgver}/aifuel-linux-amd64")
source_aarch64=("${url}/releases/download/v${pkgver}/aifuel-linux-arm64")
sha256sums_x86_64=('SKIP')
sha256sums_aarch64=('SKIP')

package() {
    install -Dm755 "${srcdir}/aifuel-linux-${CARCH/x86_64/amd64}" "${pkgdir}/usr/bin/aifuel"

    # Generate shell completions
    mkdir -p "${pkgdir}/usr/share/bash-completion/completions"
    mkdir -p "${pkgdir}/usr/share/zsh/site-functions"
    mkdir -p "${pkgdir}/usr/share/fish/vendor_completions.d"

    "${pkgdir}/usr/bin/aifuel" completion bash > "${pkgdir}/usr/share/bash-completion/completions/aifuel" 2>/dev/null || true
    "${pkgdir}/usr/bin/aifuel" completion zsh > "${pkgdir}/usr/share/zsh/site-functions/_aifuel" 2>/dev/null || true
    "${pkgdir}/usr/bin/aifuel" completion fish > "${pkgdir}/usr/share/fish/vendor_completions.d/aifuel.fish" 2>/dev/null || true
}
