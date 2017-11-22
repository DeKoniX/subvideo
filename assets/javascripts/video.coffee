$(document).ready(->
    window.scroll(0, 1000)
)

$('.chat-hide').click(->
  $('#chat').fadeOut('fast')
  $('#stream').css('max-width', '100%')
  $('#stream').css('width', '100%')
  $('#stream').css('flex-basis', 'auto')
  $('.chat-show').fadeIn('fast')
)

$('.chat-show').click(->
  $('#chat').fadeIn('fast')
  $('#stream').css('max-width', '')
  $('#stream').css('width', '')
  $('#stream').css('flex-basis', '')
  $('.chat-show').fadeOut('fast')
)
