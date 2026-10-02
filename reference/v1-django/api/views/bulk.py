from rest_framework.views import APIView
from rest_framework.response import Response
from api.utils.qr import generate_qr_base64


class BulkGenerateView(APIView):
    def post(self, request):
        items = request.data.get("items")
        if not isinstance(items, list) or not items:
            return Response({"success": False, "error": "items array required"}, status=400)
        if len(items) > 100:
            return Response({"success": False, "error": "Maximum 100 items per bulk request"}, status=400)

        results = []
        for item in items:
            content = item.get("content", "")
            size = max(50, min(int(item.get("size", 256)), 1024))
            try:
                b64 = generate_qr_base64(content, size)
                results.append({"content": content, "qr_base64": b64, "success": True})
            except Exception as e:
                results.append({"content": content, "error": str(e), "success": False})

        return Response({"success": True, "results": results})
