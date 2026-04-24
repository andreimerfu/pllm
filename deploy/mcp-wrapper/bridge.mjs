#!/usr/bin/env node
//
// bridge.mjs — runs an stdio MCP server as a child process and exposes
// it over HTTP, following the MCP streamable-HTTP transport shape.
//
// The pllm K8sAdapter injects these env vars:
//   PLLM_WRAPPER_KIND     npx | uvx
//   PLLM_WRAPPER_PACKAGE  package identifier (@scope/name or pypi-name)
//   PLLM_WRAPPER_VERSION  version string (optional; latest if unset)
//   PLLM_WRAPPER_ARGS     space-separated extra args (optional)
//
// One child per pod; many HTTP requests multiplex through that child by
// matching JSON-RPC IDs. Notifications (no id) are fire-and-forget.

import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import readline from 'node:readline';

const KIND = process.env.PLLM_WRAPPER_KIND || 'npx';
const PKG = process.env.PLLM_WRAPPER_PACKAGE;
const VERSION = process.env.PLLM_WRAPPER_VERSION || '';
const ARGS = (process.env.PLLM_WRAPPER_ARGS || '').split(' ').filter(Boolean);
const PORT = parseInt(process.env.PORT || '8000', 10);
const REQUEST_TIMEOUT_MS = parseInt(process.env.REQUEST_TIMEOUT_MS || '60000', 10);

if (!PKG) {
  console.error('[wrapper] PLLM_WRAPPER_PACKAGE is required');
  process.exit(1);
}

let cmd, cmdArgs;
switch (KIND) {
  case 'npx': {
    const ref = VERSION ? `${PKG}@${VERSION}` : PKG;
    cmd = 'npx';
    cmdArgs = ['-y', ref, ...ARGS];
    break;
  }
  case 'uvx': {
    const ref = VERSION ? `${PKG}==${VERSION}` : PKG;
    cmd = 'uvx';
    cmdArgs = [ref, ...ARGS];
    break;
  }
  default:
    console.error(`[wrapper] unknown PLLM_WRAPPER_KIND: ${KIND}`);
    process.exit(1);
}

console.error(`[wrapper] spawning: ${cmd} ${cmdArgs.join(' ')}`);
const child = spawn(cmd, cmdArgs, { stdio: ['pipe', 'pipe', 'inherit'] });

let childExited = false;
child.on('exit', (code, sig) => {
  childExited = true;
  console.error(`[wrapper] child exited code=${code} signal=${sig}`);
  // Exit the whole pod so k8s restarts us with a fresh child.
  process.exit(code ?? 1);
});
child.on('error', (err) => {
  console.error(`[wrapper] child error: ${err.message}`);
});

// Pending request map: id → resolver
const pending = new Map();

// Read child stdout line-by-line (MCP stdio frames one JSON message per line).
const rl = readline.createInterface({ input: child.stdout });
rl.on('line', (line) => {
  const trimmed = line.trim();
  if (!trimmed) return;
  let msg;
  try {
    msg = JSON.parse(trimmed);
  } catch (err) {
    console.error(`[wrapper] non-JSON stdout from child (drop): ${trimmed.slice(0, 200)}`);
    return;
  }
  if (msg.id === undefined || msg.id === null) {
    // Notification from child — no SSE channel to push on, drop.
    return;
  }
  const waiter = pending.get(String(msg.id));
  if (waiter) {
    pending.delete(String(msg.id));
    waiter.resolve(msg);
  }
});

function writeChild(body) {
  if (childExited || !child.stdin.writable) {
    throw new Error('child process not writable');
  }
  child.stdin.write(body + '\n');
}

const server = createServer(async (req, res) => {
  if (req.method === 'GET' && (req.url === '/healthz' || req.url === '/')) {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end(childExited ? 'child_exited' : 'ok');
    return;
  }
  if (req.method !== 'POST') {
    res.writeHead(405);
    res.end();
    return;
  }

  let raw = '';
  req.on('data', (c) => (raw += c));
  req.on('end', async () => {
    let msg;
    try {
      msg = JSON.parse(raw);
    } catch (err) {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        jsonrpc: '2.0',
        error: { code: -32700, message: 'parse error: ' + err.message },
      }));
      return;
    }

    // Notifications (no id): fire-and-forget.
    if (msg.id === undefined || msg.id === null) {
      try {
        writeChild(raw);
        res.writeHead(202);
        res.end();
      } catch (err) {
        res.writeHead(502);
        res.end();
      }
      return;
    }

    // Requests: forward + await response.
    const idKey = String(msg.id);
    const wait = new Promise((resolve, reject) => {
      pending.set(idKey, { resolve, reject });
    });
    try {
      writeChild(raw);
    } catch (err) {
      pending.delete(idKey);
      res.writeHead(502, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        jsonrpc: '2.0', id: msg.id,
        error: { code: -32603, message: err.message },
      }));
      return;
    }

    const timer = setTimeout(() => {
      if (pending.delete(idKey)) {
        res.writeHead(504, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32603, message: 'upstream MCP server timed out' },
        }));
      }
    }, REQUEST_TIMEOUT_MS);

    try {
      const response = await wait;
      clearTimeout(timer);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(response));
    } catch (err) {
      clearTimeout(timer);
      if (!res.headersSent) {
        res.writeHead(502, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32603, message: err.message },
        }));
      }
    }
  });
});

server.listen(PORT, () => {
  console.error(`[wrapper] listening on :${PORT}`);
});

for (const sig of ['SIGTERM', 'SIGINT']) {
  process.on(sig, () => {
    console.error(`[wrapper] ${sig} received, shutting down`);
    try { child.kill('SIGTERM'); } catch {}
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 5000).unref();
  });
}
