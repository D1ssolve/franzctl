#!/bin/sh
set -eu

# Demonstration codec. Real codecs can call Avro/Protobuf libraries or a schema
# registry. The executable receives bytes on stdin and must write bytes to
# stdout. FRANZCTL_CODEC_MODE is either "encode" or "decode".
rev
