$(window).scroll(->
  if $(document).scrollTop() > 0
      $('.scrollup').fadeIn('fast')
  else
      $('.scrollup').fadeOut('fast')
)

$('.scrollup').click(->
  window.scroll(0, 0)
)
