from django.urls import path
from api.views.redirect import DynamicRedirectView

urlpatterns = [
    path("", DynamicRedirectView.as_view()),
]
