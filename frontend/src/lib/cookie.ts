// From https://git.colormatic.org/zakarya/gist/src/branch/main/cookie.ts

// The magic cookie system is so stupid

export function setCookie(
	cname: string,
	cvalue: string,
	sameSite: "Lax" | "Strict" | "None" = "Lax"
) {
	document.cookie = cname + "=" + cvalue + ";" + "SameSite" + "=" + sameSite + ";";
}

// Credit: https://www.w3schools.com/js/js_cookies.asp
export function getCookie(cname: string): string {
	const name = cname + "=";
	const decodedCookie = decodeURIComponent(document.cookie);
	const ca = decodedCookie.split(";");
	for (let i = 0; i < ca.length; i++) {
		let c = ca[i];
		while (c.charAt(0) == " ") {
			c = c.substring(1);
		}
		if (c.indexOf(name) == 0) {
			return c.substring(name.length, c.length);
		}
	}
	return "";
}
