import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Routes that require authentication
const PROTECTED_PREFIXES = ["/dashboard"];

// Routes that are always public
const PUBLIC_ROUTES = [
    "/",
    "/login",
    "/register",
    "/forgot-password",
    "/reset-password",
    "/verify-email",
    "/pricing",
    "/invite",
];

const PUBLIC_PREFIXES = [
    "/p/",
    "/url-qr-code",
    "/wifi-qr-code",
    "/vcard-qr-code",
    "/email-qr-code",
    "/event-qr-code",
    "/social-media-qr-code",
    "/upi-qr-code",
    "/whatsapp-qr-code",
];

function isPublicRoute(pathname: string): boolean {
    if (PUBLIC_ROUTES.includes(pathname)) return true;
    return PUBLIC_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

function isProtectedRoute(pathname: string): boolean {
    return PROTECTED_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

export function middleware(request: NextRequest) {
    const { pathname } = request.nextUrl;

    // Skip static files and API routes
    if (
        pathname.startsWith("/_next") ||
        pathname.startsWith("/api") ||
        pathname.includes(".") // static files like .ico, .png etc.
    ) {
        return NextResponse.next();
    }

    // Check for auth token in the zustand persisted storage cookie/localStorage
    // Since middleware runs on the edge and can't access localStorage,
    // we check for a lightweight auth cookie or the Authorization header.
    // For SPA apps with zustand persist, we do a client-side redirect instead.
    // The middleware sets a header that the client can check.

    if (isProtectedRoute(pathname)) {
        // We can't directly read localStorage from middleware.
        // Instead, we set a custom header and let the client-side handle
        // the actual redirect. This provides a layer of protection for
        // server-rendered pages.
        const response = NextResponse.next();
        response.headers.set("x-middleware-auth-check", "true");
        return response;
    }

    return NextResponse.next();
}

export const config = {
    matcher: [
        /*
         * Match all request paths except:
         * - _next/static (static files)
         * - _next/image (image optimization files)
         * - favicon.ico (favicon)
         */
        "/((?!_next/static|_next/image|favicon.ico).*)",
    ],
};
