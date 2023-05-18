# gifthumb

Generate GIF / animated thumbnails from a video URL.

## How it works

- User requests `/thumb?url=...`
- gifthumb runs `ffprobe` to check the video at the URL
- gifthumb runs N instances of `ffmpeg` to capture frames at equal intervals
  across the whole video duration
- another `ffmpeg` instance is run to combine the frames into a gif
- gif is cached and served to user

## How to use

Copy `config.example.json` to `config.json` and run

```
go run ./main.go
```
