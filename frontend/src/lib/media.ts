import { type API } from "$lib/api";

export async function uploadMediaFiles(api: API, files: FileList | File[]): Promise<string[]> {
	const mediaIds: string[] = [];
	for (const file of Array.from(files)) {
		const mediaId = await api.uploadMedia(file);
		mediaIds.push(mediaId);
	}
	return mediaIds;
}
