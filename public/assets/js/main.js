(function(){$(window).scroll(function(){return $(document).scrollTop()>0?$(".scrollup").fadeIn("fast"):$(".scrollup").fadeOut("fast")}),$(".scrollup").click(function(){return window.scroll(0,0)}),$(".refresh").click(function(){return location.reload()}),new Clipboard(".jscopy")}).call(this);