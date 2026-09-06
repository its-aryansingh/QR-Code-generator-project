release: cd backend && python manage.py collectstatic --noinput && python manage.py migrate --noinput
web: cd backend && gunicorn qrapp.wsgi:application --config gunicorn.conf.py --bind 0.0.0.0:$PORT