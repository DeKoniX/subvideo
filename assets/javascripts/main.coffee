$(window).scroll(->
  if $(document).scrollTop() > 0
      $('.scrollup').fadeIn('fast')
  else
      $('.scrollup').fadeOut('fast')
)

$('.scrollup').click(->
  window.scroll(0, 0)
)

searchParams = new URLSearchParams(window.location.search)
if searchParams.get('page') != null
  if $('*').is('#video')
    $('html, body').animate({
      scrollTop: $("#video").offset().top
    }, 400)
