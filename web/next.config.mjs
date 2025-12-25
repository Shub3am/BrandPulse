/** @type {import('next').NextConfig} */

// `standalone` is what makes the runtime image small and what makes it runnable
// without `next` on the PATH: the build traces the modules each route actually
// imports and copies just those into .next/standalone, with its own server.js.
// Without it the image needs the whole node_modules tree and `next start`.
//
// `outputFileTracingRoot` is pinned because it decides where server.js lands and
// Next infers it from the nearest lockfile or package.json above this directory.
// There is none above web/ today, so the entrypoint is .next/standalone/server.js;
// the day anyone adds a package.json at the repo root it would silently become
// .next/standalone/web/server.js and the image would start failing to find its own
// entrypoint. Pinning it to this directory fixes the path. Nothing the server
// imports lives outside web/, so nothing is left untraced by pinning it here.
export default {
  output: "standalone",
  outputFileTracingRoot: import.meta.dirname,
};
