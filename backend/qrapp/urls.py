from django.urls import path, include

urlpatterns = [
    path("health", include("api.urls_health")),
    path("r/<str:code>", include("api.urls_redirect")),
    path("api/v1/", include("api.urls")),
]
