require "language/go"

class Gor < Formula
  desc "Real-time HTTP traffic replay tool written in Go"
  homepage "https://gortool.com"
  url "https://github.com/buger/gor/archive/v0.13.0.tar.gz"
  sha256 "e4af70afacc162200b63d66927824f64f54e3cf662c50ce37e54912565a31f23"
  head "https://github.com/buger/gor.git"

  bottle do
    cellar :any_skip_relocation
    sha256 "c382403de70a41b7445920a02051f5e82030704aaaae70cfcd4e8f401cc87f6a" => :el_capitan
    sha256 "4b76b3785584897800e87967f1af9510208faefe46f57d7bd6f8b40a7133c19b" => :yosemite
    sha256 "d186cb1566d33ab8f78215e69934f49dd96becb1c236905b4502d94399ae1974" => :mavericks
  end

  depends_on "go" => :build

  go_resource "github.com/bitly/go-hostpool" do
    url "https://github.com/bitly/go-hostpool.git",
      :revision => "d0e59c22a56e8dadfed24f74f452cea5a52722d2"
  end

  go_resource "github.com/buger/elastigo" do
    url "https://github.com/buger/elastigo.git",
      :revision => "23fcfd9db0d8be2189a98fdab77a4c90fcc3a1e9"
  end

  go_resource "github.com/google/gopacket" do
    url "https://github.com/google/gopacket.git",
      :revision => "aa09ced736460d76535444c825932a0742975f7d"
  end

  def install
    ENV["GOPATH"] = buildpath
    mkdir_p buildpath/"src/github.com/buger/"
    ln_sf buildpath, buildpath/"src/github.com/buger/gor"
    Language::Go.stage_deps resources, buildpath/"src"

    system "go", "build", "-o", "#{bin}/gor", "-ldflags", "-X main.VERSION \"#{version}\""
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gor", 1)
  end
end