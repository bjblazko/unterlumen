package de.huepattl.unterlumen.photo

import com.twelvemonkeys.imageio.plugins.jpeg.JPEGImageReaderSpi
import com.twelvemonkeys.imageio.plugins.jpeg.JPEGImageWriterSpi
import io.quarkus.logging.Log
import jakarta.enterprise.context.ApplicationScoped
import java.awt.image.BufferedImage
import java.io.File
import javax.imageio.ImageIO
import javax.imageio.spi.IIORegistry

@ApplicationScoped
class ImageScaler {

    fun scaleToWidth(source: String, width: Int, dest: String): Int {
        Log.info("scaling '$source' to a width of $width pixels to file '$dest'...")
        IIORegistry.getDefaultInstance().registerServiceProvider(JPEGImageReaderSpi())
        IIORegistry.getDefaultInstance().registerServiceProvider(JPEGImageWriterSpi())

        val originalImage = ImageIO.read(File(source))

        val aspectRatio = originalImage.height.toDouble() / originalImage.width.toDouble()
        val height = (width * aspectRatio).toInt()

        val scaledImage = BufferedImage(width, height, BufferedImage.TYPE_INT_RGB)
        val graphics = scaledImage.createGraphics()
        graphics.drawImage(originalImage, 0, 0, width, height, null)
        graphics.dispose()

        ImageIO.write(scaledImage, "jpeg", File(dest)) // FIXME other formats

        return height
    }

}