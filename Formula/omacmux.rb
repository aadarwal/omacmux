class Omacmux < Formula
  desc "Agent-first IDE built on tmux — AI agents as first-class terminal panes"
  homepage "https://github.com/aadarwal/omacmux"
  # TODO: update URL and sha256 when you create a release tag
  url "https://github.com/aadarwal/omacmux/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "PLACEHOLDER_UPDATE_WITH_REAL_SHA256"
  license "MIT"
  head "https://github.com/aadarwal/omacmux.git", branch: "master"

  depends_on :macos
  depends_on "bash"
  depends_on "tmux"
  depends_on "neovim"
  depends_on "starship"
  depends_on "eza"
  depends_on "fzf"
  depends_on "zoxide"
  depends_on "bat"
  depends_on "ripgrep"
  depends_on "fd"
  depends_on "mise"
  depends_on "gh"
  depends_on "jq"
  depends_on "tree"

  def install
    # Install everything into the Cellar prefix
    prefix.install Dir["*"]
    prefix.install Dir[".*"].reject { |f| %w[. .. .git].include?(File.basename(f)) }

    # Symlink the CLI into Homebrew's bin
    bin.install_symlink prefix/"bin/omacmux"
  end

  def caveats
    <<~EOS
      omacmux has been installed. To set up your config files, run:

        omacmux init

      This will interactively link tmux, neovim, bash, and other configs.
      Existing files will be backed up before any changes.

      To see the current state of your config links:

        omacmux status

      To check installation health:

        omacmux doctor

      For the Nerd Font (terminal icons), install separately:

        brew install --cask font-jetbrains-mono-nerd-font
    EOS
  end

  test do
    assert_match "agent-first IDE", shell_output("#{bin}/omacmux help")
  end
end
