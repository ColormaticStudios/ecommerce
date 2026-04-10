import type { API } from "$lib/api";

type ApiMethods = {
	[K in keyof API as API[K] extends (...args: never[]) => unknown ? K : never]?: API[K];
};

export function createApiStub(overrides: ApiMethods = {}): API {
	return new Proxy(overrides, {
		get(target, prop) {
			if (typeof prop !== "string") {
				return undefined;
			}
			if (prop in target) {
				return target[prop as keyof ApiMethods];
			}
			return (...args: unknown[]) => {
				throw new Error(
					`Unstubbed story API method "${prop}" called with ${JSON.stringify(args)}.`
				);
			};
		},
	}) as API;
}

export function pendingPromise<T>(): Promise<T> {
	return new Promise(() => undefined);
}
