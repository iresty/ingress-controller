#!/usr/bin/env bash

set -o errexit

function upload_test_images() {
  echo ">> Publishing test images"
  (
    # Script needs to be executed from repo root
    cd "$( dirname "$0")/.."
#    local image_dir="$(find "${GOPATH}/pkg/mod/knative.dev" -type d -name "networking*" | head -n1)/test/test_images"
    local image_dir="tmpvendor/knative.dev/networking/test/test_images"
    local docker_tag=$1
    local tag_option=""
    if [ -n "${docker_tag}" ]; then
      tag_option="--tags $docker_tag,latest"
    fi

    for yaml in $(find ${image_dir} -name '*.yaml'); do
      # Rewrite image reference to use vendor.
      #sed "s@knative.dev/networking@fhuzero/apisix-ingress-controller/tmpvendor/knative.dev/networking@g" $yaml \
      cat $yaml \
        `# ko resolve is being used for the side-effect of publishing images,` \
        `# so the resulting yaml produced is ignored.` \
        | ko resolve ${tag_option} -RBf- > /dev/null
    done
  )
}

: ${KO_DOCKER_REPO:?"You must set 'KO_DOCKER_REPO', see DEVELOPMENT.md"}

upload_test_images $@
