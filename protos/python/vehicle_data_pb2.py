# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: vehicle_data.proto
"""Generated protocol buffer code."""
from google.protobuf.internal import builder as _builder
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x12vehicle_data.proto\x12\x16telemetry.vehicle_data\x1a\x1fgoogle/protobuf/timestamp.proto\"4\n\rLocationValue\x12\x10\n\x08latitude\x18\x01 \x01(\x01\x12\x11\n\tlongitude\x18\x02 \x01(\x01\"\xde\x02\n\x05Value\x12\x16\n\x0cstring_value\x18\x01 \x01(\tH\x00\x12\x13\n\tint_value\x18\x02 \x01(\x05H\x00\x12\x14\n\nlong_value\x18\x03 \x01(\x03H\x00\x12\x15\n\x0b\x66loat_value\x18\x04 \x01(\x02H\x00\x12\x16\n\x0c\x64ouble_value\x18\x05 \x01(\x01H\x00\x12\x17\n\rboolean_value\x18\x06 \x01(\x08H\x00\x12?\n\x0elocation_value\x18\x07 \x01(\x0b\x32%.telemetry.vehicle_data.LocationValueH\x00\x12?\n\x0e\x63harging_value\x18\x08 \x01(\x0e\x32%.telemetry.vehicle_data.ChargingStateH\x00\x12?\n\x11shift_state_value\x18\t \x01(\x0e\x32\".telemetry.vehicle_data.ShiftStateH\x00\x42\x07\n\x05value\"a\n\x05\x44\x61tum\x12*\n\x03key\x18\x01 \x01(\x0e\x32\x1d.telemetry.vehicle_data.Field\x12,\n\x05value\x18\x02 \x01(\x0b\x32\x1d.telemetry.vehicle_data.Value\"s\n\x07Payload\x12+\n\x04\x64\x61ta\x18\x01 \x03(\x0b\x32\x1d.telemetry.vehicle_data.Datum\x12.\n\ncreated_at\x18\x02 \x01(\x0b\x32\x1a.google.protobuf.Timestamp\x12\x0b\n\x03vin\x18\x03 \x01(\t*\xa9\x14\n\x05\x46ield\x12\x0b\n\x07Unknown\x10\x00\x12\r\n\tDriveRail\x10\x01\x12\x0f\n\x0b\x43hargeState\x10\x02\x12\x19\n\x15\x42msFullchargecomplete\x10\x03\x12\x10\n\x0cVehicleSpeed\x10\x04\x12\x0c\n\x08Odometer\x10\x05\x12\x0f\n\x0bPackVoltage\x10\x06\x12\x0f\n\x0bPackCurrent\x10\x07\x12\x07\n\x03Soc\x10\x08\x12\x0e\n\nDCDCEnable\x10\t\x12\x08\n\x04Gear\x10\n\x12\x17\n\x13IsolationResistance\x10\x0b\x12\x11\n\rPedalPosition\x10\x0c\x12\x0e\n\nBrakePedal\x10\r\x12\x0b\n\x07\x44iState\x10\x0e\x12\n\n\x06\x44iPcbt\x10\x0f\x12\t\n\x05\x44iRpm\x10\x10\x12\x11\n\rDiTorquemotor\x10\x11\x12\x0f\n\x0b\x44iHeatsinkt\x10\x12\x12\n\n\x06\x44iVbat\x10\x13\x12\x12\n\x0e\x44iMotorcurrent\x10\x14\x12\x0c\n\x08Location\x10\x15\x12\x0c\n\x08GpsState\x10\x16\x12\x0e\n\nGpsHeading\x10\x17\x12\x16\n\x12NumBrickVoltageMax\x10\x18\x12\x13\n\x0f\x42rickVoltageMax\x10\x19\x12\x16\n\x12NumBrickVoltageMin\x10\x1a\x12\x13\n\x0f\x42rickVoltageMin\x10\x1b\x12\x14\n\x10NumModuleTempMax\x10\x1c\x12\x11\n\rModuleTempMax\x10\x1d\x12\x14\n\x10NumModuleTempMin\x10\x1e\x12\x11\n\rModuleTempMin\x10\x1f\x12\x0e\n\nRatedRange\x10 \x12\x08\n\x04Hvil\x10!\x12\x16\n\x12\x44\x43\x43hargingEnergyIn\x10\"\x12\x13\n\x0f\x44\x43\x43hargingPower\x10#\x12\x16\n\x12\x41\x43\x43hargingEnergyIn\x10$\x12\x13\n\x0f\x41\x43\x43hargingPower\x10%\x12\x12\n\x0e\x43hargeLimitSoc\x10&\x12\x16\n\x12\x46\x61stChargerPresent\x10\'\x12\x13\n\x0f\x45stBatteryRange\x10(\x12\x15\n\x11IdealBatteryRange\x10)\x12\x10\n\x0c\x42\x61tteryLevel\x10*\x12\x14\n\x10TimeToFullCharge\x10+\x12\x1e\n\x1aScheduledChargingStartTime\x10,\x12\x1c\n\x18ScheduledChargingPending\x10-\x12\x1a\n\x16ScheduledDepartureTime\x10.\x12\x1a\n\x16PreconditioningEnabled\x10/\x12\x19\n\x15ScheduledChargingMode\x10\x30\x12\x0e\n\nChargeAmps\x10\x31\x12\x17\n\x13\x43hargeEnableRequest\x10\x32\x12\x11\n\rChargerPhases\x10\x33\x12\x1d\n\x19\x43hargePortColdWeatherMode\x10\x34\x12\x18\n\x14\x43hargeCurrentRequest\x10\x35\x12\x1b\n\x17\x43hargeCurrentRequestMax\x10\x36\x12\x13\n\x0f\x42\x61tteryHeaterOn\x10\x37\x12\x18\n\x14NotEnoughPowerToHeat\x10\x38\x12\"\n\x1eSuperchargerSessionTripPlanner\x10\x39\x12\r\n\tDoorState\x10:\x12\n\n\x06Locked\x10;\x12\x0c\n\x08\x46\x64Window\x10<\x12\x0c\n\x08\x46pWindow\x10=\x12\x0c\n\x08RdWindow\x10>\x12\x0c\n\x08RpWindow\x10?\x12\x0f\n\x0bVehicleName\x10@\x12\x0e\n\nSentryMode\x10\x41\x12\x12\n\x0eSpeedLimitMode\x10\x42\x12\x13\n\x0f\x43urrentLimitMph\x10\x43\x12\x0b\n\x07Version\x10\x44\x12\x12\n\x0eTpmsPressureFl\x10\x45\x12\x12\n\x0eTpmsPressureFr\x10\x46\x12\x12\n\x0eTpmsPressureRl\x10G\x12\x12\n\x0eTpmsPressureRr\x10H\x12\x15\n\x11TmpsPressureRe1L0\x10I\x12\x15\n\x11TmpsPressureRe1L1\x10J\x12\x15\n\x11TmpsPressureRe1R0\x10K\x12\x15\n\x11TmpsPressureRe1R1\x10L\x12\x15\n\x11TmpsPressureRe2L0\x10M\x12\x15\n\x11TmpsPressureRe2L1\x10N\x12\x15\n\x11TmpsPressureRe2R0\x10O\x12\x15\n\x11TmpsPressureRe2R1\x10P\x12\x1e\n\x1aTpmsLastSeenPressureTimeFl\x10Q\x12\x1e\n\x1aTpmsLastSeenPressureTimeFr\x10R\x12\x1e\n\x1aTpmsLastSeenPressureTimeRl\x10S\x12\x1e\n\x1aTpmsLastSeenPressureTimeRr\x10T\x12\x0e\n\nInsideTemp\x10U\x12\x0f\n\x0bOutsideTemp\x10V\x12\x12\n\x0eSeatHeaterLeft\x10W\x12\x13\n\x0fSeatHeaterRight\x10X\x12\x16\n\x12SeatHeaterRearLeft\x10Y\x12\x17\n\x13SeatHeaterRearRight\x10Z\x12\x18\n\x14SeatHeaterRearCenter\x10[\x12\x17\n\x13\x41utoSeatClimateLeft\x10\\\x12\x18\n\x14\x41utoSeatClimateRight\x10]\x12\x12\n\x0e\x44riverSeatBelt\x10^\x12\x15\n\x11PassengerSeatBelt\x10_\x12\x16\n\x12\x44riverSeatOccupied\x10`\x12\x1d\n\x19PassengerSeatFoldPosition\x10\x61\x12\x17\n\x13LateralAcceleration\x10\x62\x12\x1c\n\x18LongitudinalAcceleration\x10\x63\x12\x0f\n\x0b\x43ruiseState\x10\x64\x12\x12\n\x0e\x43ruiseSetSpeed\x10\x65\x12\x16\n\x12LifetimeEnergyUsed\x10\x66\x12\x1b\n\x17LifetimeEnergyUsedDrive\x10g\x12\x1a\n\x16TractorParkBrakeStatus\x10h\x12\x1a\n\x16TrailerParkBrakeStatus\x10i\x12\x11\n\rBrakePedalPos\x10j\x12\x14\n\x10RouteLastUpdated\x10k\x12\r\n\tRouteLine\x10l\x12\x12\n\x0eMilesToArrival\x10m\x12\x14\n\x10MinutesToArrival\x10n\x12\x12\n\x0eOriginLocation\x10o\x12\x17\n\x13\x44\x65stinationLocation\x10p\x12\x0b\n\x07\x43\x61rType\x10q\x12\x08\n\x04Trim\x10r\x12\x11\n\rExteriorColor\x10s\x12\r\n\tRoofColor\x10t\x12\x0e\n\nChargePort\x10u\x12\x13\n\x0f\x43hargePortLatch\x10v\x12\x12\n\x0e\x45xperimental_1\x10w\x12\x12\n\x0e\x45xperimental_2\x10x\x12\x12\n\x0e\x45xperimental_3\x10y\x12\x12\n\x0e\x45xperimental_4\x10z\x12\x14\n\x10GuestModeEnabled\x10{\x12\x15\n\x11PinToDriveEnabled\x10|*\xbf\x01\n\rChargingState\x12\x16\n\x12\x43hargeStateUnknown\x10\x00\x12\x1b\n\x17\x43hargeStateDisconnected\x10\x01\x12\x16\n\x12\x43hargeStateNoPower\x10\x02\x12\x17\n\x13\x43hargeStateStarting\x10\x03\x12\x17\n\x13\x43hargeStateCharging\x10\x04\x12\x17\n\x13\x43hargeStateComplete\x10\x05\x12\x16\n\x12\x43hargeStateStopped\x10\x06*\x91\x01\n\nShiftState\x12\x15\n\x11ShiftStateUnknown\x10\x00\x12\x15\n\x11ShiftStateInvalid\x10\x01\x12\x0f\n\x0bShiftStateP\x10\x02\x12\x0f\n\x0bShiftStateR\x10\x03\x12\x0f\n\x0bShiftStateD\x10\x04\x12\x0f\n\x0bShiftStateN\x10\x05\x12\x11\n\rShiftStateSNA\x10\x06\x42/Z-github.com/teslamotors/fleet-telemetry/protosb\x06proto3')

_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, globals())
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'vehicle_data_pb2', globals())
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'Z-github.com/teslamotors/fleet-telemetry/protos'
  _FIELD._serialized_start=703
  _FIELD._serialized_end=3304
  _CHARGINGSTATE._serialized_start=3307
  _CHARGINGSTATE._serialized_end=3498
  _SHIFTSTATE._serialized_start=3501
  _SHIFTSTATE._serialized_end=3646
  _LOCATIONVALUE._serialized_start=79
  _LOCATIONVALUE._serialized_end=131
  _VALUE._serialized_start=134
  _VALUE._serialized_end=484
  _DATUM._serialized_start=486
  _DATUM._serialized_end=583
  _PAYLOAD._serialized_start=585
  _PAYLOAD._serialized_end=700
# @@protoc_insertion_point(module_scope)
