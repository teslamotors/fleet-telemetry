# frozen_string_literal: true
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: vehicle_error.proto

require 'google/protobuf'

require 'google/protobuf/timestamp_pb'


descriptor_data = "\n\x13vehicle_error.proto\x12\x17telemetry.vehicle_error\x1a\x1fgoogle/protobuf/timestamp.proto\"\x83\x01\n\rVehicleErrors\x12\x35\n\x06\x65rrors\x18\x01 \x03(\x0b\x32%.telemetry.vehicle_error.VehicleError\x12.\n\ncreated_at\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0b\n\x03vin\x18\x03 \x01(\t\"\xc6\x01\n\x0cVehicleError\x12.\n\ncreated_at\x18\x01 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0c\n\x04name\x18\x02 \x01(\t\x12=\n\x04tags\x18\x03 \x03(\x0b\x32/.telemetry.vehicle_error.VehicleError.TagsEntry\x12\x0c\n\x04\x62ody\x18\x04 \x01(\t\x1a+\n\tTagsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12\r\n\x05value\x18\x02 \x01(\t:\x02\x38\x01\x42/Z-github.com/teslamotors/fleet-telemetry/protosb\x06proto3"

pool = Google::Protobuf::DescriptorPool.generated_pool

begin
  pool.add_serialized_file(descriptor_data)
rescue TypeError
  # Compatibility code: will be removed in the next major version.
  require 'google/protobuf/descriptor_pb'
  parsed = Google::Protobuf::FileDescriptorProto.decode(descriptor_data)
  parsed.clear_dependency
  serialized = parsed.class.encode(parsed)
  file = pool.add_serialized_file(serialized)
  warn "Warning: Protobuf detected an import path issue while loading generated file #{__FILE__}"
  imports = [
    ["google.protobuf.Timestamp", "google/protobuf/timestamp.proto"],
  ]
  imports.each do |type_name, expected_filename|
    import_file = pool.lookup(type_name).file_descriptor
    if import_file.name != expected_filename
      warn "- #{file.name} imports #{expected_filename}, but that import was loaded as #{import_file.name}"
    end
  end
  warn "Each proto file must use a consistent fully-qualified name."
  warn "This will become an error in the next major version."
end

module Telemetry
  module VehicleError
    VehicleErrors = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("telemetry.vehicle_error.VehicleErrors").msgclass
    VehicleError = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("telemetry.vehicle_error.VehicleError").msgclass
  end
end
