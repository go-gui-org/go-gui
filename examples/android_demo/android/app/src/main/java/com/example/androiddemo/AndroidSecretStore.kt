package com.example.androiddemo

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.AtomicFile
import androidapp.SecretStore
import java.io.File
import java.io.FileNotFoundException
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import java.security.MessageDigest
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Stores go-gui secrets encrypted with an app-private Android Keystore key. */
class AndroidSecretStore(context: Context) : SecretStore {
    private val applicationContext = context.applicationContext
    private val keyAlias = "${applicationContext.packageName}.go-gui-secrets-v1"

    @Throws(Exception::class)
    override fun load(service: String, key: String): ByteArray = synchronized(storeLock) {
        val aad = associatedData(service, key)
        val atomicFile = AtomicFile(secretFile(aad))
        val blob = try {
            // readFully also restores AtomicFile's backup after an interrupted
            // write; checking only baseFile first would skip that recovery.
            atomicFile.readFully()
        } catch (_: FileNotFoundException) {
            return@synchronized ByteArray(0)
        }
        if (blob.size > MAX_BLOB_BYTES) {
            throw IOException("encrypted secret is too large")
        }

        val parsed = parseBlob(blob)
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, secretKey(), GCMParameterSpec(GCM_TAG_BITS, parsed.iv))
        cipher.updateAAD(aad)
        val plaintext = cipher.doFinal(parsed.ciphertext)
        if (plaintext.isEmpty() || plaintext.size > MAX_SECRET_BYTES) {
            plaintext.fill(0)
            throw IOException("decrypted secret has an invalid size")
        }
        plaintext
    }

    @Throws(Exception::class)
    override fun save(service: String, key: String, value: ByteArray) = synchronized(storeLock) {
        try {
            if (value.isEmpty() || value.size > MAX_SECRET_BYTES) {
                throw IllegalArgumentException("secret must contain 1 to $MAX_SECRET_BYTES bytes")
            }

            val aad = associatedData(service, key)
            val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, secretKey())
            cipher.updateAAD(aad)
            val blob = encodeBlob(cipher.iv, cipher.doFinal(value))

            val atomicFile = AtomicFile(secretFile(aad))
            val output = atomicFile.startWrite()
            try {
                output.write(blob)
                atomicFile.finishWrite(output)
            } catch (error: Exception) {
                atomicFile.failWrite(output)
                throw error
            }
        } finally {
            // gomobile supplied this Java-owned copy. Clearing it does not
            // mutate the caller's Go slice.
            value.fill(0)
        }
    }

    @Throws(Exception::class)
    override fun delete(service: String, key: String) = synchronized(storeLock) {
        AtomicFile(secretFile(associatedData(service, key))).delete()
    }

    private fun secretFile(aad: ByteArray): File {
        // noBackupFilesDir prevents restoring ciphertext without its
        // non-exportable, device-local Keystore key.
        val directory = File(applicationContext.noBackupFilesDir, STORE_DIRECTORY)
        if ((!directory.exists() && !directory.mkdirs()) || !directory.isDirectory) {
            throw IOException("could not create the secret store directory")
        }
        val digest = MessageDigest.getInstance("SHA-256").digest(aad)
        return File(directory, digest.toHex() + FILE_SUFFIX)
    }

    private fun secretKey(): SecretKey {
        val keyStore = KeyStore.getInstance(KEYSTORE_PROVIDER).apply { load(null) }
        val existing = keyStore.getKey(keyAlias, null)
        if (existing != null) {
            return existing as? SecretKey
                ?: throw GeneralSecurityException("secret store key has an unexpected type")
        }

        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE_PROVIDER)
        generator.init(
            KeyGenParameterSpec.Builder(
                keyAlias,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .setRandomizedEncryptionRequired(true)
                .build(),
        )
        return generator.generateKey()
    }

    private fun associatedData(service: String, key: String): ByteArray =
        "$service\u0000$key".toByteArray(Charsets.UTF_8)

    private fun encodeBlob(iv: ByteArray, ciphertext: ByteArray): ByteArray {
        if (iv.isEmpty() || iv.size > MAX_IV_BYTES) {
            throw GeneralSecurityException("cipher returned an invalid IV")
        }
        return MAGIC + byteArrayOf(iv.size.toByte()) + iv + ciphertext
    }

    private fun parseBlob(blob: ByteArray): EncryptedBlob {
        val headerBytes = MAGIC.size + 1
        if (blob.size < headerBytes + MIN_IV_BYTES + GCM_TAG_BYTES) {
            throw IOException("encrypted secret is truncated")
        }
        if (!blob.copyOfRange(0, MAGIC.size).contentEquals(MAGIC)) {
            throw IOException("encrypted secret has an unknown format")
        }

        val ivBytes = blob[MAGIC.size].toInt() and 0xff
        if (ivBytes !in MIN_IV_BYTES..MAX_IV_BYTES) {
            throw IOException("encrypted secret has an invalid IV")
        }
        val ciphertextStart = headerBytes + ivBytes
        if (blob.size < ciphertextStart + GCM_TAG_BYTES) {
            throw IOException("encrypted secret is truncated")
        }
        return EncryptedBlob(
            iv = blob.copyOfRange(headerBytes, ciphertextStart),
            ciphertext = blob.copyOfRange(ciphertextStart, blob.size),
        )
    }

    private fun ByteArray.toHex(): String = joinToString(separator = "") { byte ->
        "%02x".format(byte.toInt() and 0xff)
    }

    private data class EncryptedBlob(val iv: ByteArray, val ciphertext: ByteArray)

    private companion object {
        val storeLock = Any()
        val MAGIC = byteArrayOf('G'.code.toByte(), 'G'.code.toByte(), 'S'.code.toByte(), 1)

        const val KEYSTORE_PROVIDER = "AndroidKeyStore"
        const val CIPHER_TRANSFORMATION = "AES/GCM/NoPadding"
        const val STORE_DIRECTORY = "go-gui-secrets-v1"
        const val FILE_SUFFIX = ".secret"
        const val MAX_SECRET_BYTES = 2560
        const val MIN_IV_BYTES = 12
        const val MAX_IV_BYTES = 32
        const val GCM_TAG_BITS = 128
        const val GCM_TAG_BYTES = GCM_TAG_BITS / 8
        const val MAX_BLOB_BYTES = 4 + 1 + MAX_IV_BYTES + MAX_SECRET_BYTES + GCM_TAG_BYTES
    }
}
