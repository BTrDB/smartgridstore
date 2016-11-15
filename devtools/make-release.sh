#!/bin/bash
VERSION=3.4.4
if [[ "$(basename $PWD)" = "devtools" ]]
then
  cd ..
fi
mkdir smartgridstore
rsync -a bin smartgridstore/
rsync -a templates smartgridstore/
mkdir smartgridstore/units
rsync -a cluster-info.sh smartgridstore/
rsync -a README.md smartgridstore/
rsync -a LICENSE smartgridstore/
tar cvf $VERSION.tar smartgridstore
gzip -9 $VERSION.tar
mv $VERSION.tar.gz $VERSION
