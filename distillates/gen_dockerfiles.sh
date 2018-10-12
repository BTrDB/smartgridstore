#!/bin/bash

set -ex

rm -rf generated_container*

# build apifrontend just to get the version
pushd ../tools/apifrontend
go build
version=$(./apifrontend -version)
popd

# build mfgen
pushd ../mfgen
go build
MFGEN=$PWD/mfgen
popd

# this creates the container directory for a specific distillate
for alg in $(ls)
do
  if [[ ${alg} = *.sh ]]
  then
    echo "skipping ${alg}"
    continue
  fi

  mkdir generated_container_${alg}

  pushd ${alg}
  go build -v
  cp ${alg} ../generated_container_${alg}
  popd

  pushd generated_container_${alg}
  echo -e "FROM ubuntu:bionic\nADD ${alg} /\nENTRYPOINT [\"./${alg}\"]" > Dockerfile
  echo -e "#!/bin/bash\ndocker build -t btrdb/dev-distil-${alg}:${version} ." > rebuild.sh
  echo -e "docker push btrdb/dev-distil-${alg}:${version}" >> rebuild.sh
  echo -e "#!/bin/bash\ndocker build -t btrdb/distil-${alg}:${version} ." > prod-rebuild.sh
  echo -e "docker push btrdb/distil-${alg}:${version}" >> prod-rebuild.sh
  chmod a+x rebuild.sh
  chmod a+x prod-rebuild.sh
  popd

done
