import { resolve } from "$app/paths";
import { cmsHref, isExternalHref } from "$lib/cms";

export function cmsRenderHref(url: string): string {
	const href = cmsHref(url);
	if (isExternalHref(href)) {
		return href;
	}
	const [pathWithQuery, hash = ""] = href.split("#", 2);
	const [pathname, search = ""] = pathWithQuery.split("?", 2);
	let resolved = resolve(pathname as "/");
	if (search) {
		resolved += `?${search}`;
	}
	if (hash) {
		resolved += `#${hash}`;
	}
	return resolved;
}

export function cmsRenderTarget(url: string): string | undefined {
	return isExternalHref(cmsHref(url)) ? "_blank" : undefined;
}

export function cmsRenderRel(url: string): string | undefined {
	return isExternalHref(cmsHref(url)) ? "noreferrer noopener" : undefined;
}
