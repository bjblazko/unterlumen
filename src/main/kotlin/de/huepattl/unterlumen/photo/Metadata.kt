package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.types.Dimensions
import de.huepattl.unterlumen.photo.types.Exposure

data class Metadata(
    val dimensions: Dimensions? = null,
    val exposure: Exposure? = null,
)