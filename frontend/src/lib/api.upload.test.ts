import { afterEach, expect, test, vi } from "vitest";
import { API } from "./api";

const apiBase = "https://api.example.test";

afterEach(() => {
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
});

test("uploads media in tus chunks using the server's returned offset", async () => {
	const chunkSize = 5 * 1024 * 1024;
	const file = new File([new Uint8Array(chunkSize + 2)], "photo.png", {
		type: "image/png",
		lastModified: 42,
	});
	const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
		if (init?.method === "POST") {
			return new Response(null, {
				status: 201,
				headers: { Location: "/api/v1/media/uploads/upload-1" },
			});
		}
		const offset = new Headers(init?.headers).get("Upload-Offset");
		return new Response(null, {
			status: 204,
			headers: { "Upload-Offset": offset === "0" ? String(chunkSize) : String(file.size) },
		});
	});

	await expect(new API(apiBase).uploadMedia(file)).resolves.toBe("upload-1");

	const patchCalls = fetchMock.mock.calls.filter(([, init]) => init?.method === "PATCH");
	expect(patchCalls).toHaveLength(2);
	expect(new Headers(patchCalls[0][1]?.headers).get("Upload-Offset")).toBe("0");
	expect((patchCalls[0][1]?.body as Blob).size).toBe(chunkSize);
	expect(new Headers(patchCalls[1][1]?.headers).get("Upload-Offset")).toBe(String(chunkSize));
	expect((patchCalls[1][1]?.body as Blob).size).toBe(2);
});

test("resumes a stored upload from its server offset instead of creating a new upload", async () => {
	const file = new File([new Uint8Array(3)], "photo.png", {
		type: "image/png",
		lastModified: 42,
	});
	const location = `${apiBase}/api/v1/media/uploads/upload-1`;
	const key = `media-upload:${apiBase}:${file.name}:${file.size}:${file.lastModified}:${file.type}`;
	const entries = new Map([[key, location]]);
	vi.stubGlobal("window", {
		localStorage: {
			getItem: (storageKey: string) => entries.get(storageKey) ?? null,
			setItem: (storageKey: string, value: string) => entries.set(storageKey, value),
			removeItem: (storageKey: string) => entries.delete(storageKey),
		},
	});
	const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
		if (init?.method === "HEAD") {
			return new Response(null, { status: 200, headers: { "Upload-Offset": "2" } });
		}
		return new Response(null, { status: 204, headers: { "Upload-Offset": "3" } });
	});

	await expect(new API(apiBase).uploadMedia(file)).resolves.toBe("upload-1");

	expect(fetchMock.mock.calls.map(([, init]) => init?.method)).toEqual(["HEAD", "PATCH"]);
	expect(new Headers(fetchMock.mock.calls[1][1]?.headers).get("Upload-Offset")).toBe("2");
	expect(entries.has(key)).toBe(false);
});
