#!/usr/bin/env python3
"""Check bytes in debug_missing.bin"""

data = open('disposable/debug_missing.bin', 'rb').read()
print('File size:', len(data))
print('Hex (first 100):', data[:100].hex())

# Try to find the em-dash
idx = data.find(b'\xe2\x80\x94')
print('Em-dash at:', idx)
# Try to find the copyright symbol
idx2 = data.find(b'\xc2\xa9')
print('Copyright at:', idx2)
