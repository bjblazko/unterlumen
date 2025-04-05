package de.huepattl.unterlumen.photo

import io.quarkus.test.junit.QuarkusTest
import jakarta.inject.Inject
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import java.io.File

@QuarkusTest
class PhotoStoreTest {

    @Inject
    lateinit var store: PhotoStore

    @Test
    fun `store Fujifilm photo`() {
        //given
        val source = javaClass.classLoader.getResource("images/fujifilm_x-t50_bird.jpeg")?.file

        // when
        val photo = store.digest(source!!)

        // then
        assertNotNull(photo)
        assertTrue(File(photo?.handle).exists())
        assertNotNull(photo.metadata)
        assertEquals(2500, photo.metadata?.exposure?.iso)
    }

    @Test
    fun `store iPhone photo`() {
        //given
        val source = javaClass.classLoader.getResource("images/apple_iphone6s_drachenfels-geolocation.jpeg")?.file

        // when
        val photo = store.digest(source!!)

        // then
        assertNotNull(photo)
        assertTrue(File(photo?.handle).exists())
        assertNotNull(photo.metadata)
        assertEquals(25, photo.metadata?.exposure?.iso)
        assertEquals(50, photo.metadata?.geoLocation?.latitude?.degrees)
    }

    @Test
    fun `store Canon photo`() {
        //given
        val source = javaClass.classLoader.getResource("images/canon_200d_kronplatz.jpeg")?.file

        // when
        val photo = store.digest(source!!)

        // then
        assertNotNull(photo)
        assertTrue(File(photo.handle).exists())
        assertNotNull(photo.metadata)
        assertEquals(100, photo.metadata?.exposure?.iso)
    }

    @Test
    fun `store and retrieve`() {
        //given
        val id = store.digest(javaClass.classLoader.getResource("images/fujifilm_x-t50_bird.jpeg")?.file!!).id

        // when
        val photo = store.retrieve(id)

        // then
        assertNotNull(photo)
        assertTrue(File(photo?.handle).exists())
        assertNotNull(photo?.metadata)
        assertEquals(2500, photo?.metadata?.exposure?.iso)
    }

    @Test
    fun `get scaled from cache`() {
        //given
        val id = store.digest(javaClass.classLoader.getResource("images/fujifilm_x-t50_bird.jpeg")?.file!!).id

        // when
        val photo = store.retrieveScaled(id, 320)

        // then
        assertNotNull(photo)
        println(photo?.handle)
    }

}