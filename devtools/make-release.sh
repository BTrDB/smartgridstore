#!/bin/bash
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
tar cvf 3.4.3.tar smartgridstore
gzip -9 3.4.3.tar
