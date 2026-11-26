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
      url "https://github.com/chavisnguyen/wame/releases/download/v2.2.0/wame-darwin-arm64.tar.gz"
      sha256 "c566080a6422e465b5036e7e559b5c44bb0c20c333c64ffcf873c4848720a75c"
    end
    on_intel do
      url "https://github.com/chavisnguyen/wame/releases/download/v2.2.0/wame-darwin-amd64.tar.gz"
      sha256 "8fdcf62912977e8bbfa2db343f14282344d887a1fbba2d48a7e443c0e4ce6c9f"
    end
  end

  on_linux do
    # Prebuilt Linux binaries are x86_64 only.
    depends_on arch: :x86_64
    url "https://github.com/chavisnguyen/wame/releases/download/v2.2.0/wame-linux-amd64.tar.gz"
    sha256 "20321129ca05c57dc7cd8c300cc0a9f2275c4b68ad1abcc6f632783f16103750"
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
