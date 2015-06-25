FULLSIZE_PATHS = Dir.glob(File.expand_path("../public/images/*-fullsize.jpg", __FILE__))

def qsearch(range, max)
  size = yield(range.end)
  if size > max
    range.to_a.bsearch { |q| yield(q) >= max } || range.begin
  else
    range.end
  end
end

def compress(tmp_path, resized_path, q)
  system("sips -s format jpeg -s formatOptions #{q} #{tmp_path} --out #{resized_path}")
  # system("sips --resampleWidth #{width} -s format jpeg -s formatOptions 38 #{fullsize_path} --out #{resized_path}")
  # system("sips --setProperty format tga --resampleWidth #{width} #{fullsize_path} --out #{tmp_path}")
  size = `stat -f %z #{resized_path}`.to_f / 1024
  puts "Sips size: #{size}K"
  system("/Applications/ImageOptim.app/Contents/MacOS/ImageOptim #{resized_path}")
  size = `stat -f %z #{resized_path}`.to_f / 1024
  puts "ImageOptim size: #{size}K"
  size
end

FULLSIZE_PATHS.grep(/bubbles/).each do |fullsize_path|
  (200..2000).step(200).each do |width|
    resized_path = fullsize_path.sub("-fullsize.", "-#{width}.")
    tmp_path = fullsize_path.sub("-fullsize.jpg", "-tmp.jpg")
    # tmp_path = fullsize_path.sub("-fullsize.jpg", "-tmp.tga")

    system("convert #{fullsize_path} -resize #{width} -quality 100 #{tmp_path}")
    last_q = nil
    correct_q = qsearch(18..36, 180) do |q|
      last_q = q
      compress(tmp_path, resized_path, q)
    end
    if last_q != correct_q
      compress(tmp_path, resized_path, correct_q)
    end
    # system("convert #{fullsize_path} -resize #{width} -quality 55 -format jpg #{resized_path}")
    # puts "ImageMagick size: #{`stat -f %z #{resized_path}`.to_f / 1000}K"
    # system("convert #{fullsize_path} -resize #{width} -format tga #{tmp_path}")
    # puts "Resized size: #{`stat -f %z #{tmp_path}`.to_f / 1000}K"
    # system("cjpeg-mozjpeg3.1 -targa -quality 75 -outfile '#{resized_path}' '#{tmp_path}'")
    # puts "Mozjpeg size: #{`stat -f %z #{resized_path}`.to_f / 1000}K"
    # ImageOptim is set to preserve EXIF color profile info
    File.unlink(tmp_path)
  end
end
