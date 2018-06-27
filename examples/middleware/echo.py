#! /usr/bin/env python3
# -*- coding: utf-8 -*-

import sys
import fileinput
import binascii

# Used to find end of the Headers section
EMPTY_LINE = b'\r\n\r\n'


def log(msg):
    """
    Logging to STDERR as STDOUT and STDIN used for data transfer
    @type msg: str or byte string
    @param msg: Message to log to STDERR
    """
    try:
        msg = str(msg) + '\n'
    except:
        pass
    sys.stderr.write(msg)
    sys.stderr.flush()


def find_end_of_headers(byte_data):
    """
    Finds where the header portion ends and the content portion begins.
    @type byte_data: str or byte string
    @param byte_data: Hex decoded req or resp string
    """
    return byte_data.index(EMPTY_LINE) + 4


def process_stdin():
    """
    Process STDIN and output to STDOUT
    """
    for raw_line in fileinput.input():

        line = raw_line.rstrip()

        # Decode base64 encoded line
        decoded = bytes.fromhex(line)

        # Split into metadata and payload, the payload is headers + body
        (raw_metadata, payload) = decoded.split(b'\n', 1)

        # Split into headers and payload
        headers_pos = find_end_of_headers(payload)
        raw_headers = payload[:headers_pos]
        raw_content = payload[headers_pos:]

        log('===================================')
        request_type_id = int(raw_metadata.split(b' ')[0])
        log('Request type: {}'.format({
          1: 'Request',
          2: 'Original Response',
          3: 'Replayed Response'
        }[request_type_id]))
        log('===================================')

        log('Original data:')
        log(line)

        log('Decoded request:')
        log(decoded)

        encoded = binascii.hexlify(raw_metadata + b'\n' + raw_headers + raw_content).decode('ascii')
        log('Encoded data:')
        log(encoded)

        sys.stdout.write(encoded + '\n')

if __name__ == '__main__':
    process_stdin()
