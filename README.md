Пиши урлы с указанием схемы в файл указаный в переменной CONFIG
Корректные примеры:
https://weurwiueyruweyriwueyriwuer.ru
http://www.google.com:80
https://flant.com:443
http://localhost:9190
http://127.0.0.1:9190/metric
http://127.0.0.1

Экспортер слушает на порту: 9190 в локейшене /metric
Метрика выглядит так:
mini_goga_time{url="http://www.google.com:80",code="200",error="0"} 385
последняя цифра - время ответа в милисекундах

Есть локейшен /health для проб здоровья контейнера
