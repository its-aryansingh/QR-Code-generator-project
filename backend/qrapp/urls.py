from django.http import JsonResponse
from django.urls import include, path


def custom_handler404(request, exception=None):
    return JsonResponse(
        {"success": False, "error": "Endpoint not found", "code": "not_found", "status": 404},
        status=404,
    )


def custom_handler500(request):
    return JsonResponse(
        {"success": False, "error": "Internal server error", "code": "server_error", "status": 500},
        status=500,
    )


handler404 = custom_handler404
handler500 = custom_handler500

urlpatterns = [
    path("health", include("api.urls_health")),
    path("r/<str:code>", include("api.urls_redirect")),
    path("api/v1/", include("api.urls")),
]
