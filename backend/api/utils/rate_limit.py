import threading
from datetime import date
from django.conf import settings
from django.db import IntegrityError, transaction
from django.db.models import F
from django.utils import timezone

_lock = threading.Lock()
_cache: dict[str, int] = {}
_day: str | None = None


def _today() -> str:
    return str(date.today())


def _rl_check_and_increment(ip: str, limit: int) -> tuple[bool, int, bool]:
    global _cache, _day
    today = _today()
    with _lock:
        if today != _day:
            _cache = {}
            _day = today
        count = _cache.get(ip)
        if count is None:
            return False, 0, False
        if count >= limit:
            return False, count, True
        _cache[ip] = count + 1
        return True, count + 1, True


def _rl_seed(ip: str, count: int):
    global _cache, _day
    today = _today()
    with _lock:
        if today != _day:
            _cache = {}
            _day = today
        if count > _cache.get(ip, 0):
            _cache[ip] = count


def _async_increment(ip: str):
    def run():
        from api.models import FreeTierUsage

        FreeTierUsage.objects.filter(ip_address=ip, date=date.today()).update(
            qr_count=F("qr_count") + 1,
            last_used=timezone.now(),
        )

    threading.Thread(target=run, daemon=True).start()


def check_free_tier_limit(request) -> dict:
    from api.utils.ip import get_client_ip
    ip = get_client_ip(request)
    limit = settings.FREE_TIER_DAILY_LIMIT

    allowed, count, hit = _rl_check_and_increment(ip, limit)
    if hit:
        if allowed:
            _async_increment(ip)
        return {"allowed": allowed, "remaining": max(0, limit - count)}

    # Cold path. ORM queries keep the local SQLite fallback and PostgreSQL deployments
    # on the same code path.
    from api.models import FreeTierUsage

    today = date.today()
    now = timezone.now()
    with transaction.atomic():
        try:
            usage = FreeTierUsage.objects.select_for_update().get(ip_address=ip, date=today)
        except FreeTierUsage.DoesNotExist:
            try:
                with transaction.atomic():
                    usage = FreeTierUsage.objects.create(
                        ip_address=ip,
                        date=today,
                        qr_count=1,
                        first_used=now,
                        last_used=now,
                    )
                new_count = 1
            except IntegrityError:
                usage = FreeTierUsage.objects.select_for_update().get(ip_address=ip, date=today)
                usage.qr_count = int(usage.qr_count or 0) + 1
                usage.last_used = now
                usage.save(update_fields=["qr_count", "last_used"])
                new_count = int(usage.qr_count)
        else:
            usage.qr_count = int(usage.qr_count or 0) + 1
            usage.last_used = now
            usage.save(update_fields=["qr_count", "last_used"])
            new_count = int(usage.qr_count)

    _rl_seed(ip, int(new_count))
    if new_count > limit:
        return {"allowed": False, "remaining": 0}
    return {"allowed": True, "remaining": limit - int(new_count)}
