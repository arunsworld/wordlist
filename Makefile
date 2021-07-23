TARGETS := .
LD_FLAGS := -s -w

build:
	pack build arunsworld/words:latest \
		 --default-process website \
     	 --env "BP_GO_TARGETS=${TARGETS}" \
     	 --env "BP_GO_BUILD_LDFLAGS=${LD_FLAGS}" \
     	 --buildpack gcr.io/paketo-buildpacks/go \
     	 --builder paketobuildpacks/builder:tiny 

push:
	docker push arunsworld/words:latest

run:
	docker run --rm -d -p 6123:80 -v $PWD/db.db:/tmp/db.db arunsworld/words:latest -db=/tmp/db.db -port=80
