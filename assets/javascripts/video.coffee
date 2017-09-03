$(document).ready(->
    window.scroll(0, 1000)
)

$('.chat-hide').click(->
  $('#twitch-chat').fadeOut('fast')
  $('#twitch-stream').css('max-width', '100%')
  $('#twitch-stream').css('width', '100%')
  $('#twitch-stream').css('flex-basis', 'auto')
  $('.chat-show').fadeIn('fast')
)

$('.chat-show').click(->
  $('#twitch-chat').fadeIn('fast')
  $('#twitch-stream').css('max-width', '75%')
  $('#twitch-stream').css('width', '100%')
  $('#twitch-stream').css('flex-basis', '75%')
  $('.chat-show').fadeOut('fast')
)
