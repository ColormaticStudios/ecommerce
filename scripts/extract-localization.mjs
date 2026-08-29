#!/usr/bin/env node

import { readFile, readdir, stat, writeFile } from "node:fs/promises";
import { dirname, relative, resolve } from "node:path";
import process from "node:process";

const repositoryRoot = resolve(
  dirname(new URL(import.meta.url).pathname),
  "..",
);
const outputPath = resolve(repositoryRoot, "defaults/localization.en-US.json");
const sourceRoots = [
  resolve(repositoryRoot, "frontend/src/routes"),
  resolve(repositoryRoot, "frontend/src/lib/components"),
  resolve(repositoryRoot, "frontend/src/lib/admin"),
];

async function filesUnder(path) {
  const result = [];
  for (const name of await readdir(path)) {
    const child = resolve(path, name);
    const info = await stat(child);
    if (info.isDirectory()) result.push(...(await filesUnder(child)));
    else if (name.endsWith(".svelte") || name.endsWith(".ts"))
      result.push(child);
  }
  return result;
}

function decodeLiteral(raw, quote) {
  return JSON.parse(`${quote}${raw}${quote}`);
}

function lineAt(source, index) {
  return source.slice(0, index).split("\n").length;
}

function ownerFor(key) {
  if (key.startsWith("checkout.")) return "checkout";
  if (key.startsWith("admin.")) return "admin";
  if (key.startsWith("errors.")) return "http";
  if (key.startsWith("storefront.account.")) return "account";
  return "storefront";
}

function addEntry(entries, key, sourceText, file, line) {
  const separator = key.indexOf(".");
  if (separator < 1 || !sourceText) return;
  const namespace = key.slice(0, separator);
  const storedKey = key.slice(separator + 1);
  const previous = entries.get(key);
  if (previous && previous.source_text !== sourceText) {
    throw new Error(
      `${key} has conflicting source text in ${previous.file} and ${file}`,
    );
  }
  entries.set(key, {
    namespace,
    key: storedKey,
    source_text: sourceText,
    owner_domain: ownerFor(key),
    file,
    line,
  });
}

function extractCalls(entries, source, file) {
  const translationPattern =
    /\.translate\(\s*(["'])((?:\\.|(?!\1)[\s\S])*)\1\s*,\s*(["'])((?:\\.|(?!\3)[\s\S])*)\3/g;
  for (const match of source.matchAll(translationPattern)) {
    addEntry(
      entries,
      decodeLiteral(match[2], match[1]),
      decodeLiteral(match[4], match[3]),
      file,
      lineAt(source, match.index),
    );
  }
  const pluralPattern =
    /\.plural\(\s*(["'])((?:\\.|(?!\1)[\s\S])*)\1[\s\S]*?\{([\s\S]*?)\}\s*\)/g;
  for (const match of source.matchAll(pluralPattern)) {
    const baseKey = decodeLiteral(match[2], match[1]);
    const forms = match[3];
    const formPattern =
      /(zero|one|two|few|many|other)\s*:\s*(["'])((?:\\.|(?!\2)[\s\S])*)\2/g;
    for (const form of forms.matchAll(formPattern)) {
      addEntry(
        entries,
        `${baseKey}.${form[1]}`,
        decodeLiteral(form[3], form[2]),
        file,
        lineAt(source, match.index),
      );
    }
  }
}

async function extractStorefrontDefaults(entries) {
  const path = resolve(repositoryRoot, "defaults/storefront.json");
  let parsed;
  try {
    parsed = JSON.parse(await readFile(path, "utf8"));
  } catch (error) {
    if (error?.code === "ENOENT") return;
    throw error;
  }
  function visit(value, segments) {
    if (typeof value === "string" && value.trim()) {
      addEntry(
        entries,
        `storefront.defaults.${segments.join(".")}`,
        value.trim(),
        "defaults/storefront.json",
        1,
      );
      return;
    }
    if (Array.isArray(value))
      value.forEach((entry, index) =>
        visit(entry, [...segments, String(index)]),
      );
    else if (value && typeof value === "object") {
      for (const [key, entry] of Object.entries(value))
        visit(entry, [...segments, key]);
    }
  }
  visit(parsed, []);
}

async function extractBackendSourceCatalog(entries) {
  const path = resolve(repositoryRoot, "internal/migrations/migrations.go");
  const source = await readFile(path, "utf8");
  const pattern =
    /\{Namespace:\s*("(?:\\.|[^"\\])*")\s*,\s*Key:\s*("(?:\\.|[^"\\])*")\s*,\s*SourceText:\s*("(?:\\.|[^"\\])*")\s*,\s*OwnerDomain:\s*("(?:\\.|[^"\\])*")\}/g;
  for (const match of source.matchAll(pattern)) {
    const namespace = JSON.parse(match[1]);
    const key = JSON.parse(match[2]);
    const sourceText = JSON.parse(match[3]);
    const ownerDomain = JSON.parse(match[4]);
    const qualified = `${namespace}.${key}`;
    const previous = entries.get(qualified);
    if (previous && previous.source_text !== sourceText) {
      throw new Error(
        `${qualified} has conflicting source text in ${previous.file} and internal/migrations/migrations.go`,
      );
    }
    entries.set(qualified, {
      namespace,
      key,
      source_text: sourceText,
      owner_domain: ownerDomain,
      file: "internal/migrations/migrations.go",
      line: lineAt(source, match.index),
    });
  }
}

const entries = new Map();
for (const root of sourceRoots) {
  for (const path of await filesUnder(root)) {
    const source = await readFile(path, "utf8");
    extractCalls(entries, source, relative(repositoryRoot, path));
  }
}
await extractStorefrontDefaults(entries);
await extractBackendSourceCatalog(entries);

const output = `${JSON.stringify(
  {
    locale: "en-US",
    generated_by: "scripts/extract-localization.mjs",
    messages: [...entries.values()].sort((left, right) =>
      `${left.namespace}.${left.key}`.localeCompare(
        `${right.namespace}.${right.key}`,
      ),
    ),
  },
  null,
  2,
)}\n`;

if (process.argv.includes("--check")) {
  const current = await readFile(outputPath, "utf8");
  if (current !== output) {
    console.error(
      "Localization baseline is stale. Run: bun run localization:extract",
    );
    process.exit(1);
  }
  console.log(`Localization baseline is current (${entries.size} messages).`);
} else {
  await writeFile(outputPath, output);
  console.log(
    `Wrote ${entries.size} messages to ${relative(repositoryRoot, outputPath)}.`,
  );
}
