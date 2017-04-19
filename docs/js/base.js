$(document).ready(function () {
    $('img[alt="YOUTUBE"]').each(function () {
        var id = $(this).attr('src').split('/')[$(this).attr('src').split('/').length - 1].split("=")[1];
        var video = '<iframe style="width: 100%;height: 450px;" src="https://www.youtube.com/embed/' + id + '" frameborder="0" allowfullscreen></iframe>';
        $(this).replaceWith(video);
    });
});