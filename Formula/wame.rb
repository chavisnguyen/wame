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
      url "https://github.com/chavisnguyen/wame/releases/download/v2.0.0/wame-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/chavisnguyen/wame/releases/download/v2.0.0/wame-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    # Prebuilt Linux binaries are x86_64 only.
    depends_on arch: :x86_64
    url "https://github.com/chavisnguyen/wame/releases/download/v2.0.0/wame-linux-amd64.tar.gz"
    sha256 "0000000000000000000000000000000000000000000000000000000000000000"
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
