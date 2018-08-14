all: generate

########################
# docker_gen
########################

gen_img := gcr.io/istio-testing/protoc:2018-06-12
pwd := $(shell pwd)
mount_dir := /src
repo_dir := github.ibm.com/istio-research/multicluster-roadmap/api
repo_mount := $(mount_dir)/github.ibm.com/istio-research/multicluster-roadmap/api
docker_gen := docker run --rm -v $(pwd):$(repo_mount) -w $(mount_dir) $(gen_img) -I$(repo_dir)
out_path = .

########################
# protoc_gen_gogo*
########################

gogo_plugin_prefix := --gogo_out=plugins=grpc,
gogofast_plugin_prefix := --gogofast_out=plugins=grpc,
gogoslick_plugin_prefix := --gogoslick_out=plugins=grpc,

########################
# protoc_gen_python
########################

protoc_gen_python_prefix := --python_out=,
protoc_gen_python_plugin := $(protoc_gen_python_prefix):$(repo_dir)/python/istio_api

comma := ,
empty:=
space := $(empty) $(empty)

importmaps := \
	gogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto \
	google/protobuf/any.proto=github.com/gogo/protobuf/types \
	google/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor \
	google/protobuf/duration.proto=github.com/gogo/protobuf/types \
	google/protobuf/struct.proto=github.com/gogo/protobuf/types \
	google/protobuf/timestamp.proto=github.com/gogo/protobuf/types \
	google/protobuf/wrappers.proto=github.com/gogo/protobuf/types \
	google/rpc/status.proto=github.com/gogo/googleapis/google/rpc \
	google/rpc/code.proto=github.com/gogo/googleapis/google/rpc \
	google/rpc/error_details.proto=github.com/gogo/googleapis/google/rpc \

# generate mapping directive with M<proto>:<go pkg>, format for each proto file
mapping_with_spaces := $(foreach map,$(importmaps),M$(map),)
gogo_mapping := $(subst $(space),$(empty),$(mapping_with_spaces))

gogo_plugin := $(gogo_plugin_prefix)$(gogo_mapping):$(out_path)
gogofast_plugin := $(gogofast_plugin_prefix)$(gogo_mapping):$(out_path)
gogoslick_plugin := $(gogoslick_plugin_prefix)$(gogo_mapping):$(out_path)

########################
# protoc_gen_docs
########################

protoc_gen_docs_plugin := --docs_out=warnings=true,mode=html_fragment_with_front_matter:$(repo_dir)/

#####################
# Generation Rules
#####################

generate: \
	generate-mc-go

#####################
# mc/...
#####################

config_mc_path := multicluster/v1alpha1
config_mc_protos := $(shell find $(config_mc_path) -type f -name '*.proto' | sort)
config_mc_pb_gos := $(config_mc_protos:.proto=.pb.go)
config_mc_pb_doc := $(config_mc_path)/istio.multicluster.v1alpha1.pb.html

generate-mc-go: $(config_mc_pb_gos) $(config_mc_pb_doc)

$(config_mc_pb_gos) $(config_mc_pb_doc): $(config_mc_protos)
	## Generate multicluster/v1alpha1/*.pb.go + $(config_mc_pb_doc)
	@$(docker_gen) $(gogofast_plugin) $(protoc_gen_docs_plugin)$(config_mc_path) $^

clean-mc:
	rm -f $(config_mc_pb_gos)
	rm -f $(config_mc_pb_doc)

#####################
# Cleanup
#####################

clean: 	clean-mc
