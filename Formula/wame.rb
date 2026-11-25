class Wame < Formula
  desc "Lightweight WhatsApp MCP server (~15MB RAM) powered by Go"
  homepage "https://github.com/chavisnguyen/wame"
  license "MIT"

  # NOTE: `url`/`sha256` below are rewritten on every published Release by
  # the "Release assets" workflow (.github/workflows/release.yml). Do not edit
  # by hand. (`version` is inferred from the tag in the URL.)
  livecheck do
    url :homepage
    strategy :github_latest
  end

  on_macos do
    on_arm do
      url "https://github.com/chavisnguyen/wame/releases/download/v2.1.0/wame-darwin-arm64.tar.gz"
      sha256 "091951213015578b0920b619ddba4d63766fe4bf6963daf48d6e6d83614b5545"
    end
    on_intel do
      url "https://github.com/chavisnguyen/wame/releases/download/v2.1.0/wame-darwin-amd64.tar.gz"
      sha256 "0a6d59e1bfca4c5b5ec3647b61c4a748aac56caaebf0011f84ba8c79f24c0de0"
    end
  end

  on_linux do
    # Prebuilt Linux binaries are x86_64 only.
    depends_on arch: :x86_64
    url "https://github.com/chavisnguyen/wame/releases/download/v2.1.0/wame-linux-amd64.tar.gz"
    sha256 "127cffd52f9998651bbd242209c5e455b014fb60711751fa6a637b7c8d5e7acd"
  end

  def install
    bin.install "wame"
  end

  service do
    run [opt_bin/"wame", "-sse", "-port", "3026"]
    keep_alive true
    log_path var/"log/wame.log"
    error_log_path var/"log/wame.log"
  end

  def caveats
    <<~EOS
      Quickstart:
        1. Configure AI clients & store (~/.wame/store):
             wame setup

        2. Link your WhatsApp account (one-time QR scan):
             wame login

        3. Start the background daemon:
             brew services start wame

        4. Verify health:
             wame doctor

      Optional: install ffmpeg (`brew install ffmpeg`) for PTT voice notes.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/wame --version")
  end
end
