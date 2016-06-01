#!/usr/bin/env ruby
# encoding: utf-8
while data = STDIN.gets # continiously read line from STDIN
  next unless data
  data = data.chomp # remove end of line symbol
  
  decoded = [data].pack("H*") # decode base64 encoded request
  
  # dedoded value is raw HTTP payload, example:
  #   
  #   POST /post HTTP/1.1
  #   Content-Length: 7
  #   Host: www.w3.org
  #
  #   a=1&b=2"
  
  encoded = decoded.unpack("H*").first # encoding back to base64
  
  # Emit request back
  # You can skip this if want to filter out request
  STDOUT.puts encoded
end
