Пиши урлы с указанием схемы в файл указаный в переменной CONFIG</br>
Корректные примеры:</br>
https://weurwiueyruweyriwueyriwuer.ru</br>
http://www.google.com:80</br>
https://flant.com:443</br>
http://localhost:9190</br>
http://127.0.0.1:9190/metric</br>
http://127.0.0.1</br>
</br>
Экспортер слушает на порту: 9190 в локейшене /metric</br>
Метрика выглядит так:</br>
mini_goga_time{url="http://www.google.com:80",code="200",error="0"} 385</br>
последняя цифра - время ответа в милисекундах</br>
</br>
Есть локейшен /health для проб здоровья контейнера</br>
