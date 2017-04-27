#!/usr/bin/env node
const readline = require("readline");
const StringDecoder = require("string_decoder").StringDecoder

const rl = readline.createInterface({
  input: process.stdin
});

var ignoreIds = new Set();
var ignoreAddresses = "/api";
const decoder = new StringDecoder("utf8");

function convertHexString(hex) {
  var bytes = [];
  for (var i = 0; i < hex.length - 1; i += 2) {
    bytes.push(parseInt(hex.substr(i, 2), 16));
  }
  return decoder.write(Buffer.from(bytes));
}

function log(output) {
	console.error("===================");
	console.error(output);
}

function shouldOutputLine(request) {
  const components = request.split("\n");
  const header = components[0].split(" ");
  const type = parseInt(header[0]);
  const tag = header[1];

  if (type === 3) {
    return true;
  }
  if (type === 1) {
    // Check if it's oauth
    const endpoint = components[1].split(" ")[1];
    if (!endpoint.startsWith(ignoreAddresses)) {
      ignoreIds.add(tag);
      return false;
    }
  } else if (type === 2) {
    if (ignoreIds.has(tag)) {
      ignoreIds.delete(tag);
      return false;
    }
  }
  return true;
}

rl.on("line", (input) => {
  const str = convertHexString(input);
  console.log(input);
  if (shouldOutputLine(str)) {
    log(str);
	}
});
