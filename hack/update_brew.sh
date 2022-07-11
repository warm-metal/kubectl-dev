#!/usr/bin/env bash

set -e
set -x

[[ $# -lt 1 ]] && echo "version to be released is required" && exit 2
[[ ! "$1" =~ ^v[0-9].[0-9].[0-9]$ ]] && echo "version must be in the form of v0.0.0" && exit 2

VERSION=$1
TAG=$1
BRANCH=${VERSION:1:${#VERSION}-2}0

if [[ $(git branch -l ${BRANCH}) == "" ]]; then
  echo "branch ${BRANCH} not found"
  exit 1
else
  git checkout ${BRANCH}
fi

COMMIT=$(git rev-parse ${BRANCH})

[[ ! "${VERSION}" =~ ^v[0-9].[0-9].[0-9]$ ]] && echo "You must checkout the version branch" && exit 2

SOURCE=https://github.com/warm-metal/kubectl-dev/archive/${VERSION}.tar.gz
SOURCE_CHECKSUM=$(curl -skL "${SOURCE}" | shasum -ba 256 - | awk '{ print $1 }')
BREW_HOME=$(brew config | grep HOMEBREW_PREFIX | awk '{ print $2 }')
FORMULA_HOME=${BREW_HOME}/Library/Taps/warm-metal/homebrew-rc
FORMULA_FILE=${FORMULA_HOME}/Formula/kubectl-dev.rb
mkdir -p $(dirname ${FORMULA_FILE})

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

set +e
brew list warm-metal/rc/kubectl-dev > /dev/null
if [ $? -eq 0 ]; then
  echo "Uninstall formula"
  brew uninstall warm-metal/rc/kubectl-dev
fi
set -e

echo "${FORMULA_WO_BOTTLE}" > "${FORMULA_FILE}"
echo "Install from source"
brew install warm-metal/rc/kubectl-dev
brew uninstall warm-metal/rc/kubectl-dev

echo "Build bottle"
brew install --build-bottle warm-metal/rc/kubectl-dev

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
git commit -m "kubectl-dev ${VERSION}"
git push origin main
popd

set +x
set +e
