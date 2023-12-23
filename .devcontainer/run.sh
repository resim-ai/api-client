# Copyright 2023 ReSim, Inc.
#
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file or at
# https://opensource.org/licenses/MIT.

DIRPATH="/workspaces/${PWD##*/}"

docker run -it \
       --platform linux/amd64 \
       -p 8080:8080 \
       -p 443:443 \
       --volume $(pwd):$DIRPATH \
       --volume root-home:/root \
       --volume /var/run/docker.sock:/var/run/docker.sock \
       public.ecr.aws/resim/api-client:latest /bin/bash -c "cd $DIRPATH; $SHELL"