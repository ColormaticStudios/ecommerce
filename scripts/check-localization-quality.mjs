#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

const repositoryRoot = resolve(
  dirname(new URL(import.meta.url).pathname),
  "..",
);
const policy = JSON.parse(
  await readFile(
    resolve(repositoryRoot, "defaults/localization-policy.json"),
    "utf8",
  ),
);
const failures = [];

function lineAt(source, index) {
  return source.slice(0, index).split("\n").length;
}

const catalogs = new Map();
for (const required of policy.required_locales) {
  const catalog = JSON.parse(
    await readFile(resolve(repositoryRoot, required.catalog), "utf8"),
  );
  if (catalog.locale !== required.code) {
    failures.push(
      `${required.catalog}: expected locale ${required.code}, found ${catalog.locale}`,
    );
  }
  const keys = new Set(
    catalog.messages.map((message) => `${message.namespace}.${message.key}`),
  );
  catalogs.set(required.code, keys);
  for (const namespace of policy.critical_namespaces) {
    if (![...keys].some((key) => key.startsWith(`${namespace}.`))) {
      failures.push(
        `${required.catalog}: no translations found for critical namespace ${namespace}`,
      );
    }
  }
}

const sourceCatalog =
  catalogs.get(policy.required_locales[0]?.code) ?? new Set();
for (const required of policy.required_locales.slice(1)) {
  const keys = catalogs.get(required.code) ?? new Set();
  for (const key of sourceCatalog) {
    if (
      policy.critical_namespaces.some((namespace) =>
        key.startsWith(`${namespace}.`),
      ) &&
      !keys.has(key)
    ) {
      failures.push(`${required.catalog}: missing required translation ${key}`);
    }
  }
}

const allowedLiterals = new Set(policy.allowed_frontend_literals);
for (const file of policy.covered_frontend_files) {
  const source = await readFile(resolve(repositoryRoot, file), "utf8");
  if (file.endsWith(".svelte")) {
    const markup = source
      .replace(/<script[\s\S]*?<\/script>/g, "")
      .replace(/<style[\s\S]*?<\/style>/g, "");
    for (const match of markup.matchAll(/>\s*([A-Za-z][^<>{}]*?)\s*</g)) {
      const literal = match[1].trim();
      if (literal && !allowedLiterals.has(literal)) {
        failures.push(
          `${file}:${lineAt(markup, match.index)} hard-coded visible text: ${JSON.stringify(literal)}`,
        );
      }
    }
    for (const match of markup.matchAll(
      /\b(?:aria-label|placeholder|title|alt)=(['"])([^'"{}]*[A-Za-z][^'"{}]*)\1/g,
    )) {
      const literal = match[2].trim();
      if (literal && !allowedLiterals.has(literal)) {
        failures.push(
          `${file}:${lineAt(markup, match.index)} hard-coded user-facing attribute: ${JSON.stringify(literal)}`,
        );
      }
    }
  } else if (file.endsWith(".ts")) {
    for (const match of source.matchAll(/return\s+(['"])([A-Za-z][^'"]*)\1/g)) {
      failures.push(
        `${file}:${lineAt(source, match.index)} hard-coded returned text: ${JSON.stringify(match[2])}`,
      );
    }
  }
}

const accountSource = await readFile(
  resolve(repositoryRoot, "internal/httpapi/account.go"),
  "utf8",
);
if (!accountSource.includes('MessageKey: "errors." + code')) {
  failures.push(
    "internal/httpapi/account.go: problem responses must derive message_key from the stable error code",
  );
}
for (const file of policy.covered_backend_files) {
  const source = await readFile(resolve(repositoryRoot, file), "utf8");
  for (const match of source.matchAll(
    /problemError\([^\n]*?"([a-z][a-z0-9_]*)"/g,
  )) {
    const key = `errors.${match[1]}`;
    if (!sourceCatalog.has(key)) {
      failures.push(
        `${file}:${lineAt(source, match.index)} missing source-catalog error key ${key}`,
      );
    }
  }
}

if (failures.length) {
  console.error(
    `Localization quality check failed (${failures.length} issues):`,
  );
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(
  `Localization quality check passed (${policy.covered_frontend_files.length} frontend files, ${policy.covered_backend_files.length} backend files, ${policy.required_locales.length} required locales).`,
);
