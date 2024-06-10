#!/bin/sh

# Generates protobuf in python, golang and ruby

# python
python3 -m grpc_tools.protoc -I$PROTO_DIR --python_out=$PROTO_DIR/python $PROTO_DIR/*.proto
python3 -m grpc_tools.protoc -I$PROTO_DIR --grpc_python_out=$PROTO_DIR/python $PROTO_DIR/*_service.proto

# golang
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $PROTO_DIR/*.proto

# ruby
grpc_tools_ruby_protoc -I $PROTO_DIR --ruby_out=$PROTO_DIR/ruby/ --grpc_out=$PROTO_DIR/ruby/ $PROTO_DIR/*.proto
