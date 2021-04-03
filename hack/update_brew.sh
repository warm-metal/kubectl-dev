#!/usr/bin/env bash

set -e
set -x

BRANCH=$(git branch --show-current)
COMMIT=$(git rev-parse ${BRANCH})
VERSION=v${BRANCH}

[[ ! "${VERSION}" =~ ^v[0-9].[0-9].[0-9]$ ]] && echo "You must checkout the version branch" && exit 2

SOURCE=https://github.com/warm-metal/kubectl-dev/archive/${VERSION}.tar.gz
SOURCE_CHECKSUM=$(curl -skL "${SOURCE}" | shasum -ba 256 - | awk '{ print $1 }')
BREW_HOME=$(brew config | grep HOMEBREW_PREFIX | awk '{ print $2 }')
FORMULA_HOME=${BREW_HOME}/Library/Taps/warm-metal/homebrew-rc
FORMULA_FILE=${FORMULA_HOME}/Formula/kubectl-dev.rb

FORMULA_WO_BOTTLE=$(cat <<-EOF
class KubectlDev < Formula
  desc "Kubectl plugin to support devlopment activities in k8s clusters"
  homepage "https://github.com/warm-metal/kubectl-dev"
  url "${SOURCE}"
  sha256 "${SOURCE_CHECKSUM}"
  license "Apache-2.0"

  depends_on "go" => :build
  depends_on "kubectl"

  def install
    system "go", "build", *std_go_args,
      "-ldflags",
      "-X github.com/warm-metal/kubectl-dev/pkg/release.Version=${VERSION}" \\
      " -X github.com/warm-metal/kubectl-dev/pkg/release.Commit=${COMMIT}",
      "./cmd/dev"
  end

  test do
    system bin/"kubectl-dev", "version"
  end
end
EOF
)

brew list warm-metal/rc/kubectl-dev > /dev/null
if [ $? -eq 0 ]; then
  echo "Uninstall formula"
  brew uninstall warm-metal/rc/kubectl-dev
fi

echo "Build bottle"
brew install --build-bottle warm-metal/rc/kubectl-dev
echo "${FORMULA_WO_BOTTLE}" > "${FORMULA_FILE}"
BOTTLE=$(brew bottle warm-metal/rc/kubectl-dev | grep sha256 | awk '{$1=$1};1')

Formula=$(cat <<-EOF
class KubectlDev < Formula
  desc "Kubectl plugin to support devlopment activities in k8s clusters"
  homepage "https://github.com/warm-metal/kubectl-dev"
  url "${SOURCE}"
  sha256 "${SOURCE_CHECKSUM}"
  license "Apache-2.0"

  bottle do
    root_url "https://github.com/warm-metal/homebrew-rc/releases/download/kubectl-dev-${VERSION}"
    ${BOTTLE}
  end

  depends_on "go" => :build
  depends_on "kubectl"

  def install
    system "go", "build", *std_go_args,
      "-ldflags",
      "-X github.com/warm-metal/kubectl-dev/pkg/release.Version=${VERSION}" \\
      " -X github.com/warm-metal/kubectl-dev/pkg/release.Commit=${COMMIT}",
      "./cmd/dev"
  end

  test do
    system bin/"kubectl-dev", "version"
  end
end
EOF
)

echo "${Formula}" > "${FORMULA_FILE}"

pushd ${FORMULA_HOME}
git add "Formula/kubectl-dev.rb"
git commit -m "kubectl-dev v0.3.0"
git push origin main
popd

set +x
set +e
